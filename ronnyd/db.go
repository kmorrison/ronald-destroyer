package ronnyd

import (
	"fmt"
	"os"
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
	LoadConfig()
	dsn := fmt.Sprintf(
		"host=%s user=postgres password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("PG_HOST"),
		os.Getenv("PG_PASSWORD"),
		os.Getenv("PG_DBNAME"),
		"32768",
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

func GetHighwaterMessage(db *gorm.DB, channelId string) (*Message) {
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
	channelID := IsChannelIndexed(db, msg.ChannelID)
	if channelID == 0 {
		return nil, nil
	}
	author, err := PersistAuthorToDB(db, msg.Author)
	if err != nil {
		return nil, err
	}

	var existingMessage Message
	db.First(&existingMessage, "discord_id = ?", msg.ID)
	if existingMessage.ID != 0 {
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
