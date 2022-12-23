package tests

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"ronald-destroyer/ronnyd"
	"testing"
	"time"
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

func TestInsertSameMessageTwiceResultsInOneMessage(t *testing.T) {
	db := ronnyd.ConnectToDB()
	var indexedChannel ronnyd.Channel
	db.First(&indexedChannel)
	var adminAuthor ronnyd.Author
	db.First(&adminAuthor, "discord_id = ?", os.Getenv("ADMIN_DISCORD_ID"))

	discordMessage1 := &discordgo.Message{
		Content:   "index! more",
		ChannelID: fmt.Sprint(indexedChannel.DiscordID),
		GuildID:   fmt.Sprint(indexedChannel.GuildId),
		Timestamp: time.Now(),
		ID:        "1234",
		Author: &discordgo.User{
			ID:            adminAuthor.DiscordID,
			Username:      adminAuthor.Name,
			Discriminator: adminAuthor.Discriminator,
		},
	}

	persistedMessage, err := ronnyd.PersistMessageToDb(db, discordMessage1)
	if err != nil {
		t.Fail()
	}
	assert.NotNil(t, persistedMessage.ID)
	assert.Equal(t, persistedMessage.ReplayedAt, time.Time{})

	persistedMessage2, err := ronnyd.PersistMessageToDb(db, discordMessage1)
	assert.Nil(t, err)
	assert.NotNil(t, persistedMessage2)
	assert.Equal(t, persistedMessage.ID, persistedMessage2.ID)
	db.Delete(&ronnyd.Message{}, "discord_id = ?", discordMessage1.ID)
}

type MockedDiscord struct {
	mock.Mock
}

func (m *MockedDiscord) ChannelMessageSend(channelID string, content string) (*discordgo.Message, error) {
	m.Called(channelID, content)

	return &discordgo.Message{
		Content:   content,
		ChannelID: channelID,
		GuildID:   "1",
	}, nil
}

func TestSendPlayback(t *testing.T) {
	db := ronnyd.ConnectToDB()
	var message1 ronnyd.Message
	// we just happen to know which random message we're gonna select for playback
	db.Preload("Channel").Preload("Author").First(&message1, "discord_id = ?", "1053070231075029074")
	discordMessage1 := &discordgo.Message{
		Content:   message1.Content,
		ChannelID: fmt.Sprint(message1.ChannelID),
		GuildID:   "1",
	}

	discordMock := new(MockedDiscord)
	discordMock.On("ChannelMessageSend", message1.Channel.DiscordID, message1.Content).Return(discordMessage1, nil)
	messagesReplayed := ronnyd.RunPlayback(discordMock, os.Getenv("ADMIN_DISCORD_ID"))
	assert.Len(t, messagesReplayed, 1)
	assert.Less(t, time.Since(messagesReplayed[0].ReplayedAt), 5*time.Second)

	var message2 ronnyd.Message
	db.First(&message2, "discord_id = ?", message1.DiscordID)
	assert.Less(t, time.Since(message2.ReplayedAt), 5*time.Second)
	message2.ReplayedAt = time.Time{}
	db.Save(&message2)
	discordMock.AssertExpectations(t)
}

func TestWontReplayIndexCommands(t *testing.T) {
	db := ronnyd.ConnectToDB()
	var indexedChannel ronnyd.Channel
	db.First(&indexedChannel)
	var adminAuthor ronnyd.Author
	db.First(&adminAuthor, "discord_id = ?", os.Getenv("ADMIN_DISCORD_ID"))

	discordMessage1 := &discordgo.Message{
		Content:   "index! more",
		ChannelID: fmt.Sprint(indexedChannel.DiscordID),
		GuildID:   fmt.Sprint(indexedChannel.GuildId),
		ID:        "123456",
		Timestamp: time.Now(),
		Author: &discordgo.User{
			ID:            adminAuthor.DiscordID,
			Username:      adminAuthor.Name,
			Discriminator: adminAuthor.Discriminator,
		},
	}

	_, err := ronnyd.PersistMessageToDb(db, discordMessage1)
	if err != nil {
		t.Fail()
	}

	var message1 ronnyd.Message
	// we just happen to know which random message we're gonna select for playback
	db.Preload("Channel").Preload("Author").First(&message1, "discord_id = ?", "1053070231075029074")

	discordMock := new(MockedDiscord)
	discordMock.On("ChannelMessageSend", message1.Channel.DiscordID, message1.Content).Return(discordgo.Message{}, nil)

	messagesReplayed := ronnyd.RunPlayback(discordMock, os.Getenv("ADMIN_DISCORD_ID"))
	assert.Len(t, messagesReplayed, 1)
	assert.Less(t, time.Since(messagesReplayed[0].ReplayedAt), 5*time.Second)

	var message2 ronnyd.Message
	db.First(&message2, "discord_id = ?", message1.DiscordID)
	assert.Less(t, time.Since(message2.ReplayedAt), 5*time.Second)
	message2.ReplayedAt = time.Time{}
	db.Save(&message2)
	discordMock.AssertExpectations(t)
}

func TestUpdateMessage(t *testing.T) {
	db := ronnyd.ConnectToDB()
	var indexedChannel ronnyd.Channel
	db.First(&indexedChannel)
	var adminAuthor ronnyd.Author
	db.First(&adminAuthor, "discord_id = ?", os.Getenv("ADMIN_DISCORD_ID"))

	now := time.Now()
	discordMessage1 := &discordgo.Message{
		Content:   "hi hi hi",
		ChannelID: fmt.Sprint(indexedChannel.DiscordID),
		GuildID:   fmt.Sprint(indexedChannel.GuildId),
		Timestamp: time.Now(),
		ID:        "12345",
		Author: &discordgo.User{
			ID:            adminAuthor.DiscordID,
			Username:      adminAuthor.Name,
			Discriminator: adminAuthor.Discriminator,
		},
		EditedTimestamp: &now,
	}

	persistedMessage, err := ronnyd.PersistMessageToDb(db, discordMessage1)
	if err != nil {
		t.Fail()
	}
	assert.NotNil(t, persistedMessage.ID)
	assert.Equal(t, persistedMessage.ReplayedAt, time.Time{})
	assert.Equal(t, persistedMessage.Content, "hi hi hi")

	discordMessage1.Content = "hi hi hi hi"
	discordMessage1.Timestamp = time.Now()
	err = ronnyd.UpdateMessage(db, discordMessage1)
	if err != nil {
		fmt.Println(err)
		t.Fail()
	}

	var message1 ronnyd.Message
	db.Last(&message1, "discord_id = ?", discordMessage1.ID)

	assert.Equal(t, message1.Content, "hi hi hi hi")

	db.First(&persistedMessage, "discord_id = ?", discordMessage1.ID)
	assert.Equal(t, persistedMessage.Content, "hi hi hi")
	assert.True(t, persistedMessage.EditedAt.Equal(now))
}
// TODO: Send multiple messages and assert they get batched correctly

func TestEditedMessageNoEditTimestamp(t *testing.T){
	db := ronnyd.ConnectToDB()
	var indexedChannel ronnyd.Channel
	db.First(&indexedChannel)
	var adminAuthor ronnyd.Author
	db.First(&adminAuthor, "discord_id = ?", os.Getenv("ADMIN_DISCORD_ID"))

	now := time.Now()
	discordMessage1 := &discordgo.Message{
		Content:   "hi hi hi",
		ChannelID: fmt.Sprint(indexedChannel.DiscordID),
		GuildID:   fmt.Sprint(indexedChannel.GuildId),
		Timestamp: now,
		ID:        "1234567",
		Author: &discordgo.User{
			ID:            adminAuthor.DiscordID,
			Username:      adminAuthor.Name,
			Discriminator: adminAuthor.Discriminator,
		},
	}

	_, err := ronnyd.PersistMessageToDb(db, discordMessage1)
	if err != nil {
		t.Fail()
	}

	discordMessage1.Content = "hi hi hi hi"
	discordMessage1.Timestamp = time.Now()
	discordMessage1.EditedTimestamp = nil
	err = ronnyd.UpdateMessage(db, discordMessage1)
	if err == nil {
		fmt.Println("Expected error when updating message without edited timestamp")
		t.Fail()
	}

	var message1 ronnyd.Message
	db.Last(&message1, "discord_id = ?", discordMessage1.ID)

	assert.Equal(t, message1.Content, "hi hi hi")
	assert.LessOrEqual(t, message1.EditedAt, time.Time{})
}