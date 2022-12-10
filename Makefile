.PHONY: build bot

build:
	go build -o bin/bot ./cmd/
	go build -o bin/migrate ./cmd/migrate/

bot: build
	./bin/bot