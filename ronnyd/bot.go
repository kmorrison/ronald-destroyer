package ronnyd

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const INDEX_COMMAND = "index!"

func StartBot() error {
	fmt.Println("Starting bot")

	config := ReadConfig()

	bot, err := discordgo.New("Bot " + config["private-token"].(string))
	if err != nil {
		return err
	}

	bot.AddHandler(ReadyHandler)
	bot.AddHandler(MessageHandler)
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
				anchorMessageId = persistedMessage.DiscordId
			}
		}
	}
	return nil
}

func IsIndexCommand(m *discordgo.Message) bool {
	config := ReadConfig()
	return (strings.HasPrefix(m.Content, INDEX_COMMAND) && m.Author.ID == config["admin-discord-id"].(string))
}

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	db := ConnectToDB()
	_, err := PersistMessageToDb(db, m.Message)
	if err != nil {
		fmt.Println(err)
		return
	}
	if IsIndexCommand(m.Message) {
		ScrapeChannelForMessages(s, m.ChannelID, 200, m.ID)
	}
}
