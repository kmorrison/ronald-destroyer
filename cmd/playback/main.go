package main

import (
	"os"
	"ronald-destroyer/ronnyd"
)

func main() {
	ronnyd.LoadConfig()
	d, err := ronnyd.InitDiscordSession()
	if err != nil {
		panic(err)
	}

	ronnyd.RunPlayback(
		d,
		os.Getenv("ADMIN_DISCORD_ID"),
	)
}
