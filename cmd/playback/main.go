package main

import (
	"ronald-destroyer/ronnyd"
)

func main() {
	config := ronnyd.ReadConfig()
	ronnyd.RunPlayback(
		"982055964230422608",
		config["admin-discord-id"].(string),
	)
}
