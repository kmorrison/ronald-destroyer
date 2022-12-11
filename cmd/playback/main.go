package main

import (
	"ronald-destroyer/ronnyd"
)

func main() {
	config := ronnyd.ReadConfig()
	ronnyd.SendMessagePlayback(
		config["admin-discord-id"].(string),
		"982055964230422608",
	)
}
