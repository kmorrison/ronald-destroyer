package ronnyd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Author struct {
	gorm.Model
	DiscordID     string `gorm:"uniqueIndex"`
	Name          string
	Discriminator string
}

type Channel struct {
	gorm.Model
	DiscordID string `gorm:"uniqueIndex"`
	GuildId   string
}

type Message struct {
	gorm.Model
	Content          string
	MessageTimestamp time.Time `gorm:"index"`
	DiscordID        string    `gorm:"index"`
	ChannelID        uint
	Channel          Channel
	AuthorID         uint
	Author           Author
	ReplayedAt       time.Time
	EditedAt 	     time.Time  `gorm:"index"`
}

func ConnectToDB() *gorm.DB {
	LoadConfig()
	dsn := fmt.Sprintf(
		"host=%s user=postgres password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("PGHOST"),
		os.Getenv("PGPASSWORD"),
		os.Getenv("PGDATABASE"),
		os.Getenv("PGPORT"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	return db
}

func PersistAuthorToDB(db *gorm.DB, author *discordgo.User) (*Author, error) {
	var existingAuthor Author
	db.Limit(1).Find(&existingAuthor, "discord_id = ?", author.ID)
	if existingAuthor.ID != 0 {
		return &existingAuthor, nil
	}

	newAuthor := &Author{
		Name:          author.Username,
		Discriminator: author.Discriminator,
		DiscordID:     author.ID,
	}
	result := db.Create(newAuthor)
	if result.Error != nil {
		return nil, result.Error
	}
	return newAuthor, nil
}

func PersistChannelToDB(db *gorm.DB, channelId string, guildId string) (*Channel, error) {
	var existingChannel Channel
	db.Limit(1).Find(&existingChannel, "discord_id = ?", channelId)
	if existingChannel.ID != 0 {
		return &existingChannel, nil
	}

	newChannel := &Channel{
		DiscordID: channelId,
		GuildId:   guildId,
	}
	result := db.Create(newChannel)
	if result.Error != nil {
		return nil, result.Error
	}
	return newChannel, nil
}

func GetHighwaterMessage(db *gorm.DB, channelId string) *Message {
	var highwaterMessage Message
	db.Order("message_timestamp").First(&highwaterMessage, "channel_id = ?", channelId)
	return &highwaterMessage
}

func IsChannelIndexed(db *gorm.DB, channelId string) uint {
	var existingChannel Channel
	db.First(&existingChannel, "discord_id = ?", channelId)
	return existingChannel.ID
}

func PersistMessageToDb(db *gorm.DB, msg *discordgo.Message) (*Message, error) {
	log.Default().Println("Received message: ", msg.Content, " from ", msg.Author.Username, " in ", msg.ChannelID)
	channelID := IsChannelIndexed(db, msg.ChannelID)
	if channelID == 0 {
		return nil, nil
	}
	author, err := PersistAuthorToDB(db, msg.Author)
	if err != nil {
		return nil, err
	}

	var existingMessage Message
	db.Limit(1).Find(&existingMessage, "discord_id = ?", msg.ID)
	if existingMessage.ID != 0 {
		log.Default().Println("Ignoring existing message for persist", existingMessage.ID, msg.ID)
		return &existingMessage, nil
	}
	newMessage := &Message{
		Content:          msg.Content,
		MessageTimestamp: msg.Timestamp,
		DiscordID:        msg.ID,
		ChannelID:        channelID,
		AuthorID:         author.ID,
	}
	result := db.Create(newMessage)
	log.Default().Println("Created new message", newMessage.ID, msg.ID)
	if result.Error != nil {
		return nil, result.Error
	}
	return newMessage, nil
}

func UpdateMessage(db *gorm.DB, msg *discordgo.Message) error {
	tx := db.Begin()

	var existingMessage Message
	err := db.Clauses(
		clause.Locking{Strength: "UPDATE"},
	).Last(
		&existingMessage, 
		"discord_id = ?", 
		msg.ID,
	)
	if err.Error != nil {
		log.Default().Println("Error updating message", err.Error)
		tx.Rollback()
		return err.Error
	}

	newMessage := &Message{
		Content:          msg.Content,
		MessageTimestamp: *msg.EditedTimestamp,
		DiscordID:        msg.ID,
		ChannelID:        existingMessage.ChannelID,
		AuthorID:         existingMessage.AuthorID,
	}
	result := tx.Save(newMessage)
	if result.Error != nil {
		log.Default().Println("Error updating message", result.Error)
		tx.Rollback()
		return result.Error
	}
	existingMessage.EditedAt = *msg.EditedTimestamp
	result = tx.Save(&existingMessage)
	if result.Error != nil {
		log.Default().Println("Error updating message", result.Error)
		tx.Rollback()
		return result.Error
	}
	tx.Commit()
	return nil
}

func GetMessagesForPlayback(db *gorm.DB, authorID string) map[time.Time][]*Message {
	// XXX: This algorithm looks at all messages from user, and basically does a
	// query for each one in order to group them. This is pretty inefficient,
	// probably we should be looking at the 100 most recent unreplayed messages
	var messages []*Message
	db.Preload("Author").Preload("Channel").Joins(
		"JOIN authors ON authors.id = messages.author_id",
	).Joins(
		"JOIN channels ON channels.id = messages.channel_id",
	).Where(
		"messages.replayed_at = ?", time.Time{},
	).Where(
		"messages.edited_at = ?", time.Time{},
	).Where(
		"authors.discord_id = ?",
		authorID,
	).Order("message_timestamp").Find(&messages)

	if len(messages) == 0 {
		fmt.Println("no messages found", authorID)
		return nil
	}

	var messageSessions map[time.Time][]*Message
	var startingTime time.Time
	messageSessions = make(map[time.Time][]*Message)
	for _, message := range messages {
		if IsIndexCommand(message.Content, message.Author.DiscordID) {
			continue
		}
		// decide if we should group into a new session
		if message.MessageTimestamp.Sub(startingTime) > 5*time.Minute {
			startingTime = message.MessageTimestamp
			messageSessions[startingTime] = make([]*Message, 0)
		} else if thereExistsMessageFromSomeoneElseInBetween(db, startingTime, message.MessageTimestamp, message.AuthorID, message.ChannelID) {
			startingTime = message.MessageTimestamp
			messageSessions[startingTime] = make([]*Message, 0)
		}

		messageSessions[startingTime] = append(messageSessions[startingTime], message)
	}
	return messageSessions
}

func thereExistsMessageFromSomeoneElseInBetween(db *gorm.DB, startingTime time.Time, endingTime time.Time, authorID uint, channelID uint) bool {

	var inBetweenMessage Message
	db.Find(
		&inBetweenMessage,
		"channel_id = ? AND author_id != ? AND message_timestamp > ? AND message_timestamp < ?",
		channelID,
		authorID,
		startingTime,
		endingTime,
	)

	return inBetweenMessage.ID != 0
}

func MarkMessageAsReplayed(db *gorm.DB, message *Message) error {
	result := db.Model(&message).Update("replayed_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	return nil
}
