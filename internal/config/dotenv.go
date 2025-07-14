package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type EnvVars struct {
	RabbitUrl string
	DbUrl     string
	RedisUrl  string
}

func LoadEnv() (EnvVars, error) {
	err := godotenv.Load()
	if err != nil {
		fmt.Print("Error loading .env file.")
		return EnvVars{}, err
	}

	RabbitUrl := os.Getenv("RABBIT_URL")
	DBUrl := os.Getenv("DB_URL")
	RedisUrl := os.Getenv("REDIS_URL")

	return EnvVars{
		RabbitUrl: RabbitUrl,
		DbUrl:     DBUrl,
		RedisUrl:  RedisUrl,
	}, nil
}
