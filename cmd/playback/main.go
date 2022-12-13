package main

import (
	"os"
	"ronald-destroyer/ronnyd"
)

func main() {
	ronnyd.LoadConfig()
	ronnyd.RunPlayback(
		"982055964230422608",
		os.Getenv("ADMIN_DISCORD_ID"),
	)
}
