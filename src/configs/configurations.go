package configs

import (
	"github.com/Netflix/go-env"
	"github.com/joho/godotenv"
	"log"
)

type Config struct {
	ContractAddress           	string `env:"CONTRACT_ADDRESS"`
	ContractCreationTxHash		string `env:"CONTRACT_CREATION_TX_HASH"`
	BscApiKey					string `env:"BSC_API_KEY"`
	BscNodeAddresses			string `env:"BSC_NODE_ADDRESSES"`
}

func LoadConfig(path string) *Config {
	var config = &Config{}

	err := godotenv.Load(path)
	if err != nil {
		log.Fatalln("Error loading testdata .env file", err)
	}

	// Get environment variables for Config
	_, err = env.UnmarshalFromEnviron(config)
	if err != nil {
		log.Fatal("env.UnmarshalFromEnviron config failed", err)
	}

	return config
}
