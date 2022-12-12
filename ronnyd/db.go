package ronnyd

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	DiscordID        string    `gorm:"uniqueIndex"`
	ChannelID        uint
	Channel          Channel
	AuthorID         uint
	Author           Author
	ReplayedAt       time.Time
}

func ConnectToDB() *gorm.DB {
	// TODO: Read db info from secret
	config := ReadConfig()
	dsn := fmt.Sprintf(
		"host=localhost user=postgres password=%s dbname=ronny port=32768 sslmode=disable TimeZone=UTC",
		config["postgres-password"],
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	return db
}

func PersistAuthorToDB(db *gorm.DB, author *discordgo.User) (*Author, error) {
	var existingAuthor Author
	db.First(&existingAuthor, "discord_id = ?", author.ID)
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
	db.First(&existingChannel, "discord_id = ?", channelId)
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

func PersistMessageToDb(db *gorm.DB, msg *discordgo.Message) (*Message, error) {
	author, err := PersistAuthorToDB(db, msg.Author)
	if err != nil {
		return nil, err
	}
	channel, err := PersistChannelToDB(db, msg.ChannelID, msg.GuildID)

	var existingMessage Message
	db.First(&existingMessage, "discord_id = ?", msg.ID)
	if existingMessage.ID != 0 {
		return &existingMessage, nil
	}
	newMessage := &Message{
		Content:          msg.Content,
		MessageTimestamp: msg.Timestamp,
		DiscordID:        msg.ID,
		ChannelID:        channel.ID,
		AuthorID:         author.ID,
	}
	result := db.Create(newMessage)
	if result.Error != nil {
		return nil, result.Error
	}
	return newMessage, nil
}

func MarkMessageAsReplayed(db *gorm.DB, message *Message) error {
	result := db.Model(&message).Update("replayed_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	return nil
}
