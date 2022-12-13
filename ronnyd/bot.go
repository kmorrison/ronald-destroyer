package ronnyd

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

const INDEX_COMMAND = "index!"
const DEFAULT_MESSAGES_TO_INDEX = 250

func StartBot() error {
	fmt.Println("Starting bot")

	bot, err := initDiscordSession()
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

func initDiscordSession() (*discordgo.Session, error) {
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

func IsIndexCommand(m *discordgo.Message) bool {
	LoadConfig()
	return (strings.HasPrefix(m.Content, INDEX_COMMAND) && m.Author.ID == os.Getenv("ADMIN_DISCORD_ID"))
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
	if IsIndexCommand(m.Message) {
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

		// TODO: implement "index! more" command
	}
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
		"authors.discord_id = ?",
		authorID,
	).Order("message_timestamp").Find(&messages)

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
		} else if thereExistsMessageFromSomeoneElseInBetween(db, startingTime, message.MessageTimestamp, message.AuthorID, message.ChannelID) {
			startingTime = message.MessageTimestamp
			messageSessions[startingTime] = make([]*Message, 0)
		}

		messageSessions[startingTime] = append(messageSessions[startingTime], message)
	}
	return messageSessions
}

var playbackMutex sync.Mutex

func SelectMessageGroupForPlayback(db *gorm.DB, authorID string) []*Message {
	messageMap := GetMessagesForPlayback(db, authorID)

	keys := make([]time.Time, 0, len(messageMap))
	for k := range messageMap {
		keys = append(keys, k)
	}
	randomKey := rand.Intn(len(keys))
	return messageMap[keys[randomKey]]
}

func PlaybackMessages(s Discord, db *gorm.DB, messages []*Message) {
	for _, message := range messages {
		err := MarkMessageAsReplayed(db, message)
		if err != nil {
			fmt.Println("Failed to mark message as replayed", message.ID)
			continue
		}
		msg, err := s.ChannelMessageSend(message.Channel.DiscordID, message.Content)
		if err != nil {
			fmt.Println("Error sending message", err)
			return
		}
		fmt.Println(msg)

		time.Sleep(1 * time.Second)
	}
}

func RunPlayback(channelID string, targetID string) {
	db := ConnectToDB()

	playbackMutex.Lock()
	defer playbackMutex.Unlock()

	bot, err := initDiscordSession()
	if err != nil {
		fmt.Println("Error starting bot", err)
		return
	}
	// TODO: Make sure we haven't replayed a message from target inside some cooldown period
	messages := SelectMessageGroupForPlayback(db, targetID)
	PlaybackMessages(bot, db, messages)
}

//define discord interface so it can be mocked for testing
type Discord interface {
	ChannelMessageSend(channelID string, content string) (*discordgo.Message, error)
}
