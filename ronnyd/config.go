package ronnyd

import (
	"github.com/joho/godotenv"
)

func LoadConfig() {
	godotenv.Load()
	godotenv.Load(".env.public")
}