package main

import (
	"encoding/json"
	"io/ioutil" //it will be used to help us read our config.json file.

	"ronald-destroyer/ronnyd"
)

func main() {
	db := ronnyd.ConnectToDB()

	var authors []*ronnyd.Author
	db.Table("authors").Find(&authors)
	jsonAuthors, err := json.Marshal(authors)
	if err != nil {
		panic("Unable to serialize authors as json")
	}
	ioutil.WriteFile("testuitl/fixtures/authors.json", jsonAuthors, 0644)

	var channels []*ronnyd.Channel
	db.Table("channels").Find(&channels)
	jsonChannels, err := json.Marshal(channels)
	if err != nil {
		panic("Unable to serialize channels as json")
	}
	ioutil.WriteFile("testuitl/fixtures/channels.json", jsonChannels, 0644)

	var messages []*ronnyd.Message
	db.Table("messages").Find(&messages)
	jsonMessages, err := json.Marshal(messages)
	if err != nil {
		panic("Unable to serialize messages as json")
	}
	ioutil.WriteFile("testuitl/fixtures/messages.json", jsonMessages, 0644)
}
