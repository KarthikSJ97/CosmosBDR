package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// CosmosBDR server port
	Port int

	// Postgres DB
	PostgresDBUserName      string
	PostgresDBPassword      string
	PostgresDBName          string
	PostgresDBHost          string
	PostgresDBPort          int
	PostgresDBSSL           bool
	PostgresSchemaDirectory string

	// SeaWeedFs storage
	SeaweedEndpoint  string
	SeaweedRegion    string
	SeaweedAccessKey string
	SeaweedSecretKey string
	SeaweedBucket    string

	// Temporal workflow orchestartor
	TemporalGRPCHost string
	TemporalGRPCPort int
	TemporalUIHost   string
	TemporalUIPort   int

	// Other
	EncryptionKey        string // 32 bytes hex or string for AES-256
	CompressionAlgorithm string
}

func LoadConfig() *Config {
	// Attempt to load .env file (ignores error if running inside Docker where env vars are injected directly)
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found or failed to load, relying on system environment variables")
	}

	return &Config{
		// CosmosBDR server port
		Port: getEnvInt("PORT", 8080),

		// Postgres DB
		PostgresDBUserName:      getEnvString("POSTGRES_DB_USERNAME", "postgres"),
		PostgresDBPassword:      getEnvString("POSTGRES_DB_PASSWORD", "postgres"),
		PostgresDBName:          getEnvString("POSTGRES_DB_NAME", "cosmosbdr"),
		PostgresDBHost:          getEnvString("POSTGRES_DB_HOST", "localhost"),
		PostgresDBPort:          getEnvInt("POSTGRES_DB_PORT", 5432),
		PostgresDBSSL:           getEnvBool("POSTGRES_DB_SSL", false),
		PostgresSchemaDirectory: getEnvString("POSTGRES_DB_SCHEMA_DIRECTORY", "./src/db/schema"),

		// SeaWeedFs storage
		SeaweedEndpoint:  getEnvString("SEAWEED_ENDPOINT", "http://localhost:8333"),
		SeaweedRegion:    getEnvString("SEAWEED_REGION", "region"),
		SeaweedAccessKey: getEnvString("SEAWEED_ACCESS_KEY", "dummy"),
		SeaweedSecretKey: getEnvString("SEAWEED_SECRET_KEY", "dummy"),
		SeaweedBucket:    getEnvString("SEAWEED_BUCKET", "bucket"),

		// Temporal workflow orchestartor
		TemporalGRPCHost: getEnvString("TEMPORAL_GRPC_HOST", "localhost"),
		TemporalGRPCPort: getEnvInt("TEMPORAL_GRPC_PORT", 7233),

		// Other
		EncryptionKey:        getEnvString("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"), // 32-char default for dev
		CompressionAlgorithm: getEnvString("COMPRESSION_ALGORITHM", "gzip"),
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

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}
