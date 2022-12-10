package main

import (
	"ronald-destroyer/ronnyd"
)

func main() {
	db := ronnyd.ConnectToDB()
	db.Debug()
	db.AutoMigrate(&ronnyd.Author{}, &ronnyd.Channel{}, &ronnyd.Message{})
}