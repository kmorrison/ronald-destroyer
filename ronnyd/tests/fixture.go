package tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"
	"os"

	"ronald-destroyer/ronnyd"
)

func LoadDevFixtures(m *testing.M) {
	ronnyd.LoadConfig()
	db := ronnyd.ConnectToDB()
	var authors []*ronnyd.Author
	db.Table("authors").Find(&authors)
	if len(authors) > 0 {
		panic("Authors table is not empty at start of test run")
	}

	file, err := ioutil.ReadFile(os.Getenv("PROJ_ROOT") + "testutil/fixtures/authors.json")
	if err != nil {
		fmt.Println(err)
		panic("Could not read authors file")
	}
	var authorPayload []map[string]interface{}
	json.Unmarshal(file, &authorPayload)
	newAuthorIDMapping := make(map[float64]uint)
	for _, author := range authorPayload {
		insertedAuthor := ronnyd.Author{
			DiscordID: author["DiscordID"].(string),
			Name:     author["Name"].(string),
			Discriminator:     author["Discriminator"].(string),
		}
		db.Create(&insertedAuthor)
		newAuthorIDMapping[author["ID"].(float64)] = insertedAuthor.ID
	}

	file, err = ioutil.ReadFile(os.Getenv("PROJ_ROOT") + "testutil/fixtures/channels.json")
	if err != nil {
		fmt.Println(err)
		panic("Could not read channels file")
	}
	var channelPayload []map[string]interface{}
	json.Unmarshal(file, &channelPayload)
	newChannelIDMapping := make(map[float64]uint)
	for _, channel := range channelPayload {
		insertedChannel := ronnyd.Channel{
			DiscordID: channel["DiscordID"].(string),
			GuildId:     channel["GuildId"].(string),
		}
		db.Create(&insertedChannel)
		newChannelIDMapping[channel["ID"].(float64)] = insertedChannel.ID
	}

	file, err = ioutil.ReadFile(os.Getenv("PROJ_ROOT") + "testutil/fixtures/messages.json")
	if err != nil {
		fmt.Println(err)
		panic("Could not read channels file")
	}
	var messagePayload []map[string]interface{}
	json.Unmarshal(file, &messagePayload)
	for _, message := range messagePayload {
		replayedAt, err := time.Parse(time.RFC3339, message["ReplayedAt"].(string))
		if err != nil {
			panic(err)
		}
		if replayedAt.Year() < 1970 {
			// Hack because serializing then deserializing a time.Time{} gives you not time.Time{}
			replayedAt = time.Time{}
		}
		messageTimestamp, err := time.Parse(time.RFC3339, message["MessageTimestamp"].(string))
		if err != nil {
			panic(err)
		}
		insertedMessage := ronnyd.Message{
			DiscordID: message["DiscordID"].(string),
			Content: message["Content"].(string),
			ReplayedAt: replayedAt,
			AuthorID: newAuthorIDMapping[message["AuthorID"].(float64)],
			ChannelID: newChannelIDMapping[message["ChannelID"].(float64)],
			MessageTimestamp: messageTimestamp,
		}
		db.Create(&insertedMessage)
	}
}
