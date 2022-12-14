package tests

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
	"ronald-destroyer/ronnyd"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMain(m *testing.M) {
	ronnyd.LoadConfig()
	LoadDevFixtures(m)
	os.Exit(m.Run())
}

func TestDatabaseIsNotEmpty(t *testing.T) {
	// Mostly we're testing here that the test fixtures loaded correctly
	db := ronnyd.ConnectToDB()
	var authors []*ronnyd.Author
	db.Table("authors").Find(&authors)
	if len(authors) == 0 {
		t.Fail()
	}
}

type MockedDiscord struct {
	mock.Mock
}

func (m *MockedDiscord) ChannelMessageSend(channelID string, content string) (*discordgo.Message, error) {
	m.Called(channelID, content)
	
	return &discordgo.Message{
		Content: content,
		ChannelID: channelID,
		GuildID: "1",
	}, nil
}

func TestSendPlayback(t *testing.T) {
	rand.Seed(1234)
	db := ronnyd.ConnectToDB()
	var message1, message2 ronnyd.Message
	// we just happen to know which random message we're gonna select for playback
	db.Preload("Channel").Preload("Author").First(&message1, "id = ?", 101)
	discordMessage1 := &discordgo.Message{
		Content: message1.Content,
		ChannelID: fmt.Sprint(message1.ChannelID),
		GuildID: "1",
	}
	db.Preload("Channel").Preload("Author").First(&message2, "id = ?", 100)
	discordMessage2 := &discordgo.Message{
		Content: message1.Content,
		ChannelID: fmt.Sprint(message1.ChannelID),
		GuildID: "1",
	}

	discordMock := new(MockedDiscord)
	discordMock.On("ChannelMessageSend", message1.Channel.DiscordID, message1.Content).Return(discordMessage1, nil)
	discordMock.On("ChannelMessageSend", message2.Channel.DiscordID, message2.Content).Return(discordMessage2, nil)
	messagesReplayed := ronnyd.RunPlayback(discordMock, os.Getenv("ADMIN_DISCORD_ID"))
	assert.Len(t, messagesReplayed, 2)
	assert.Less(t, time.Since(messagesReplayed[0].ReplayedAt), 5*time.Second)
	assert.Less(t, time.Since(messagesReplayed[1].ReplayedAt), 5*time.Second)
}
