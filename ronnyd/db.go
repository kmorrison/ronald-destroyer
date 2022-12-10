package ronnyd

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Author struct {
	gorm.Model
	DiscordID     string
	Name          string
	Discriminator string
}

type Channel struct {
	gorm.Model
	DiscordId string
	GuildId   string
}

type Message struct {
	gorm.Model
	Content          string
	MessageTimestamp string
	DiscordId        string
	ChannelID        uint
	Channel          Channel
	AuthorID         uint
	Author           Author
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
		DiscordId: channelId,
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
		MessageTimestamp: msg.Timestamp.String(),
		DiscordId:        msg.ID,
		ChannelID:        channel.ID,
		AuthorID:         author.ID,
	}
	result := db.Create(newMessage)
	if result.Error != nil {
		return nil, result.Error
	}
	fmt.Println(channel, author, newMessage)
	return newMessage, nil
}
