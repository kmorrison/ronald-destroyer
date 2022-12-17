package ronnyd

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

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
	lastKey := len(keys) - 1
	return messageMap[keys[lastKey]]
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
