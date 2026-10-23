package main

import (
	"fmt"
	"log"

	"cosmosbdr/src/config"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// Load environment configuration
	cfg := config.LoadConfig()

	fmt.Printf("[Config] Database URL configured: %s\n", cfg.DatabaseURL)
	fmt.Printf("[Config] SeaweedFS S3 Endpoint: %s\n", cfg.SeaweedS3Endpoint)
	
	log.Println("CosmosBDR configuration loaded successfully. Ready for stream engine implementation.")
}