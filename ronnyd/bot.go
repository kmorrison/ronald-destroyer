package ronnyd

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

const INDEX_COMMAND = "index!"

func StartBot() error {
	fmt.Println("Starting bot")

	bot, err := initDiscordSession()

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

func initDiscordSession() (*discordgo.Session, error) {
	config := ReadConfig()

	bot, err := discordgo.New("Bot " + config["private-token"].(string))
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

func thereExistsMessageFromSomeoneElseInBetween(db *gorm.DB, startingTime time.Time, endingTime time.Time, authorID uint) bool {

	var inBetweenMessage Message
	db.Find(
		&inBetweenMessage,
		"author_id != ? AND message_timestamp > ? AND message_timestamp < ?",
		authorID,
		startingTime,
		endingTime,
	)

	return inBetweenMessage.ID != 0
}

func GetMessagesForPlayback(db *gorm.DB, authorID string, channelID string) map[time.Time][]*Message {
	var messages []*Message
	db.Joins(
		"JOIN authors ON authors.id = messages.author_id",
	).Joins(
		"JOIN channels ON channels.id = messages.channel_id",
	).Where(
		"messages.replayed_at = ?", time.Time{},
	).Where("authors.discord_id = ?", authorID).Where("channels.discord_id = ?", channelID).Order("message_timestamp").Find(&messages)

	if len(messages) == 0 {
		return nil
	}

	var messageSessions map[time.Time][]*Message
	var startingTime time.Time
	messageSessions = make(map[time.Time][]*Message)
	for _, message := range messages {
		// decide if we should group into a new session
		if message.MessageTimestamp.Sub(startingTime) > 5*time.Minute {
			startingTime = message.MessageTimestamp
			messageSessions[startingTime] = make([]*Message, 0)
		} else if thereExistsMessageFromSomeoneElseInBetween(db, startingTime, message.MessageTimestamp, message.AuthorID) {
			startingTime = message.MessageTimestamp
			messageSessions[startingTime] = make([]*Message, 0)
		}

		messageSessions[startingTime] = append(messageSessions[startingTime], message)
	}
	return messageSessions
}

var playbackMutex sync.Mutex

func SelectMessageGroupForPlayback(db *gorm.DB, authorID string, channelID string) []*Message {
	messageMap := GetMessagesForPlayback(db, authorID, channelID)

	keys := make([]time.Time, 0, len(messageMap))
	for k := range messageMap {
		keys = append(keys, k)
	}
	randomKey := rand.Intn(len(keys))
	return messageMap[keys[randomKey]]
}

func PlaybackMessages(s *discordgo.Session, db *gorm.DB, channelOrUserID string, messages []*Message) {
	for _, message := range messages {
		err := MarkMessageAsReplayed(db, message)
		if err != nil {
			fmt.Println(err)
			continue
		}
		//msg, err := s.ChannelMessageSend(channelOrUserID, message.Content)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(message)

		time.Sleep(1 * time.Second)
	}
}

func RunPlayback(channelID string, targetID string) {
	db := ConnectToDB()

	playbackMutex.Lock()
	defer playbackMutex.Unlock()

	bot, err := initDiscordSession()
	if err != nil {
		fmt.Println(err)
		return
	}
	messages := SelectMessageGroupForPlayback(db, targetID, channelID)
	PlaybackMessages(bot, db, channelID, messages)
}
