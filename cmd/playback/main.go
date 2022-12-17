package main

import (
	"flag"
	"os"
	"ronald-destroyer/ronnyd"
)

func main() {
	ronnyd.LoadConfig()
	playbackTarget := flag.String(
		"target",
		os.Getenv("ADMIN_DISCORD_ID"),
		"Target user (discord_id) to playback messages for",
	)
	flag.Parse()
	d, err := ronnyd.InitDiscordSession()
	if err != nil {
		panic(err)
	}

	ronnyd.RunPlayback(
		d,
		*playbackTarget,
	)
}
