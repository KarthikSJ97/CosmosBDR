package temporal

import (
	"log"
	"strconv"

	"cosmosbdr/src/config"

	"go.temporal.io/sdk/client"
)

// InitClient establishes a connection to the Temporal server using client.Dial
func InitClient(cfg *config.Config) (client.Client, error) {
	c, err := client.Dial(client.Options{
		HostPort: cfg.TemporalGRPCHost + ":" + strconv.Itoa(cfg.TemporalGRPCPort),
	})
	if err != nil {
		return nil, err
	}

	log.Println("Successfully connected to Temporal server at localhost:7233")
	return c, nil
}
