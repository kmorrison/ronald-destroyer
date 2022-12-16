package ronnyd

import (
	"github.com/joho/godotenv"
	"os"
)

func LoadConfig() {
	err := godotenv.Load(os.Getenv("PROJ_ROOT") + ".env")
	// There will be no .env file in production, since it's getting injected by the PaaS
	if err != nil && os.Getenv("ENV") != "production" {
		panic(err)
	}

	err = godotenv.Load(os.Getenv("PROJ_ROOT") + ".env.public")
	if err != nil {
		panic(err)
	}
}
