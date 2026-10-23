package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             int
	DatabaseURL      string
	SeaweedEndpoint  string
	SeaweedRegion    string
	SeaweedAccessKey string
	SeaweedSecretKey string
	SeaweedBucket    string
	EncryptionKey    string // 32 bytes hex or string for AES-256
	TemporalHost     string
	TemporalPort     int
}

func LoadConfig() *Config {
	// Attempt to load .env file (ignores error if running inside Docker where env vars are injected directly)
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found or failed to load, relying on system environment variables")
	}

	return &Config{
		Port:             getEnvInt("PORT", 8080),
		DatabaseURL:      getEnvString("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cosmosbdr?sslmode=disable"),
		SeaweedEndpoint:  getEnvString("SEAWEED_ENDPOINT", "http://localhost:8333"),
		SeaweedRegion:    getEnvString("SEAWEED_REGION", "region"),
		SeaweedAccessKey: getEnvString("SEAWEED_ACCESS_KEY", "dummy"),
		SeaweedSecretKey: getEnvString("SEAWEED_SECRET_KEY", "dummy"),
		SeaweedBucket:    getEnvString("SEAWEED_BUCKET", "bucket"),
		EncryptionKey:    getEnvString("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"), // 32-char default for dev
		TemporalHost:     getEnvString("TEMPORAL_HOST", "localhost"),
		TemporalPort:     getEnvInt("TEMPORAL_PORT", 7233),
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
