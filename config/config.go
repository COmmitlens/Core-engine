package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/tkanos/gonfig"
)

type Configuration struct {
	Port                   string `env:"PORT"`
	Dburl                  string `env:"DB_URL"`
	DbHost                 string `env:"DB_HOST"`
	DbPort                 string `env:"DB_PORT"`
	DbName                 string `env:"DB_NAME"`
	DbUser                 string `env:"DB_USER"`
	DbPassword             string `env:"DB_PASSWORD"`
	JWTSecret              string `env:"JWT_SECRET"`
	PrimaryEmail           string `env:"PRIMARY_EMAIL"`
	PrimaryEmailPassword   string `env:"PRIMARY_EMAIL_PASSWORD"`
	FrontendUrl            string `env:"FRONTEND_URL"`
	GitHubAppID            int64  `env:"GITHUB_APP_ID"`
	GitHubPrivateKeyPath   string `env:"GITHUB_PRIVATE_KEY_PATH"`
	AzureOpenAIEndpoint    string `env:"AZURE_OPENAI_ENDPOINT"`
	AzureOpenAIKey         string `env:"AZURE_OPENAI_KEY"`
	AzureOpenAIModel       string `env:"AZURE_OPENAI_MODEL"`
	AzureEmbeddingEndpoint string `env:"AZURE_EMBEDDING_ENDPOINT"`
	RedisAddr              string `env:"REDIS_ADDR"`
	AiBackendUrl           string `env:"AI_BACKEND_URL"`
}

func GetConfig() Configuration {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("error loading dotenv file", err)
	}
	configuration := Configuration{}
	err = gonfig.GetConf("", &configuration)
	if err != nil {
		fmt.Println("error in config:", err)
	}
	return configuration
}
