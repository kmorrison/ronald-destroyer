package ronnyd

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const INDEX_COMMAND = "index!"
const DEFAULT_MESSAGES_TO_INDEX = 250

func StartBot() error {
	fmt.Println("Starting bot")

	bot, err := InitDiscordSession()
	if err != nil {
		return err
	}

	bot.AddHandler(ReadyHandler)
	bot.AddHandler(MessageHandler)
	bot.AddHandler(EditHandler)
	err = bot.Open()
	if err != nil {
		return err
	}
	defer bot.Close()

	// Some stolen code so that the bot hangs until it receives an interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	fmt.Println("Graceful shutdown")

	return nil
}

func InitDiscordSession() (*discordgo.Session, error) {
	LoadConfig()

	bot, err := discordgo.New("Bot " + os.Getenv("DISCORD_PRIVATE_TOKEN"))
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func ReadyHandler(s *discordgo.Session, r *discordgo.Ready) {
	fmt.Println("Bot is ready")
}

func ScrapeChannelForMessages(s *discordgo.Session, channelID string, maxMessages int, anchorMessageId string) error {
	db := ConnectToDB()
	messagesPersisted := 0
	for messagesPersisted < maxMessages {
		messages, err := s.ChannelMessages(
			channelID,
			int(math.Min(float64(maxMessages-messagesPersisted), 100)),
			anchorMessageId, "", "")
		if err != nil {
			return err
		}
		for _, message := range messages {
			persistedMessage, err := PersistMessageToDb(db, message)
			if err != nil {
				return err
			} else {
				messagesPersisted++
				anchorMessageId = persistedMessage.DiscordID
			}
		}
	}
	return nil
}

func IsIndexCommand(content string, authorID string) bool {
	LoadConfig()
	return (strings.HasPrefix(content, INDEX_COMMAND) && authorID == os.Getenv("ADMIN_DISCORD_ID"))
}

func EditHandler(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	db := ConnectToDB()
	err := UpdateMessage(db, m.Message)
	if err != nil {
		log.Default().Println(err)
		return
	}
}

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	db := ConnectToDB()
	// NOTE: May not persist message if channel not indexed
	_, err := PersistMessageToDb(db, m.Message)
	if err != nil {
		fmt.Println(err)
		return
	}
	if IsIndexCommand(m.Message.Content, m.Author.ID) {
		fullCommand := strings.Split(m.Content, " ")
		switch {
		case len(fullCommand) == 1:
			{
				PersistChannelToDB(db, m.ChannelID, m.GuildID)
				ScrapeChannelForMessages(s, m.ChannelID, DEFAULT_MESSAGES_TO_INDEX, m.ID)
			}
		case len(fullCommand) == 2:
			{
				highWaterMark := m.ID
				messagesToIndex, err := strconv.Atoi(fullCommand[1])
				if err != nil {

					if fullCommand[1] == "more" {
						highwaterMessage := GetHighwaterMessage(db, m.ChannelID)
						highWaterMark = highwaterMessage.DiscordID
						messagesToIndex = DEFAULT_MESSAGES_TO_INDEX
					} else {
						return
					}
				}
				PersistChannelToDB(db, m.ChannelID, m.GuildID)
				ScrapeChannelForMessages(s, m.ChannelID, messagesToIndex, highWaterMark)
			}
		}
	}
}

//define discord interface so it can be mocked for testing
type Discord interface {
	ChannelMessageSend(channelID string, content string) (*discordgo.Message, error)
}
