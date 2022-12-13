package ronnyd

import (
	"github.com/joho/godotenv"
	"os"
)

func LoadConfig() {
	err := godotenv.Load(os.Getenv("PROJ_ROOT") + ".env")
	if err != nil {
		panic(err)
	}

	err = godotenv.Load(os.Getenv("PROJ_ROOT") + ".env.public")
	if err != nil {
		panic(err)
	}
}
