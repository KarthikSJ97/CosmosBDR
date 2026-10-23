package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              int
	DatabaseURL       string
	SeaweedS3Endpoint string
	SeaweedAccessKey  string
	SeaweedSecretKey  string
	EncryptionKey     string // 32 bytes hex or string for AES-256
	TemporalHost      string
	TemporalPort      int
}

func LoadConfig() *Config {
	// Attempt to load .env file (ignores error if running inside Docker where env vars are injected directly)
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found or failed to load, relying on system environment variables")
	}

	return &Config{
		Port:              getEnvInt("PORT", 8080),
		DatabaseURL:       getEnvString("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cosmosbdr?sslmode=disable"),
		SeaweedS3Endpoint: getEnvString("SEAWEED_S3_ENDPOINT", "http://localhost:8333"),
		SeaweedAccessKey:  getEnvString("SEAWEED_ACCESS_KEY", "dummy"),
		SeaweedSecretKey:  getEnvString("SEAWEED_SECRET_KEY", "dummy"),
		EncryptionKey:     getEnvString("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"), // 32-char default for dev
		TemporalHost:      getEnvString("TEMPORAL_HOST", "localhost"),
		TemporalPort:      getEnvInt("TEMPORAL_PORT", 7233),
	}
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
