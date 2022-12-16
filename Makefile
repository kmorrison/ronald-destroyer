.PHONY: build bot playback test

build:
	go build -o bin/bot ./cmd/
	go build -o bin/migrate ./cmd/migrate/
	go build -o bin/playback ./cmd/playback/
	go build -o bin/devdump ./cmd/devdump/

bot: build
	./bin/bot

playback: build
	./bin/playback

test: build
	docker exec local-pg2 psql -U postgres -c "DROP DATABASE IF EXISTS test"
	docker exec local-pg2 psql -U postgres -c "CREATE DATABASE test"
	PGDATABASE=test ./bin/migrate
	PROJ_ROOT=`pwd`/ PGDATABASE=test ENV=test go test -v ./...