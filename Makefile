.PHONY: build bot playback

build:
	go build -o bin/bot ./cmd/
	go build -o bin/migrate ./cmd/migrate/
	go build -o bin/playback ./cmd/playback/

bot: build
	./bin/bot

playback: build
	./bin/playback