package ronnyd

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sort"
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

	bot, err := InitDiscordSession()
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

var playbackMutex sync.Mutex

func SelectMessageGroupForPlayback(db *gorm.DB, authorID string) []*Message {
	messageMap := GetMessagesForPlayback(db, authorID)

	keys := make([]time.Time, 0, len(messageMap))
	for k := range messageMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Before(keys[j])
	})
	randomKey := rand.Intn(len(keys))
	return messageMap[keys[randomKey]]
}

func PlaybackMessages(s Discord, db *gorm.DB, messages []*Message) []*Message {
	var messagesReplayed []*Message
	for _, message := range messages {
		err := MarkMessageAsReplayed(db, message)
		if err != nil {
			fmt.Println("Failed to mark message as replayed", message.ID)
			continue
		}

		_, err = s.ChannelMessageSend(message.Channel.DiscordID, message.Content)
		if err != nil {
			fmt.Println("Error sending message", err)
			return messagesReplayed
		}
		messagesReplayed = append(messagesReplayed, message)

		if os.Getenv("ENV") != "test" {
			time.Sleep(1 * time.Second)
		}
	}
	return messagesReplayed
}

func RunPlayback(d Discord, targetID string) []*Message {
	db := ConnectToDB()

	playbackMutex.Lock()
	defer playbackMutex.Unlock()

	// TODO: Make sure we haven't replayed a message from target inside some cooldown period
	messages := SelectMessageGroupForPlayback(db, targetID)
	messagesReplayed := PlaybackMessages(d, db, messages)
	return messagesReplayed
}

//define discord interface so it can be mocked for testing
type Discord interface {
	ChannelMessageSend(channelID string, content string) (*discordgo.Message, error)
}
