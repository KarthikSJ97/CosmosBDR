package cadence

import (
	"fmt"
	"log"
	"strconv"

	"cosmosbdr/src/config"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/client"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/tchannel"
)

const (
	ClientName     = "cosmosbdr-client"
	CadenceService = "cadence-frontend"
)

// InitClient establishes a connection and returns both the high-level client and service client
func InitClient(cfg *config.Config) (client.Client, workflowserviceclient.Interface, error) {
	hostPort := cfg.CadenceGRPCHost + ":" + strconv.Itoa(cfg.CadenceGRPCPort)

	tchannelTransport, err := tchannel.NewChannelTransport(
		tchannel.ServiceName(ClientName),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tchannel transport: %w", err)
	}

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: ClientName,
		Outbounds: yarpc.Outbounds{
			CadenceService: {
				Unary: tchannelTransport.NewSingleOutbound(hostPort),
			},
		},
	})

	if err := dispatcher.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start yarpc dispatcher: %w", err)
	}

	serviceClient := workflowserviceclient.New(dispatcher.ClientConfig(CadenceService))

	domain := "samples-domain"
	cadenceClient := client.NewClient(
		serviceClient,
		domain,
		&client.Options{},
	)

	log.Printf("Successfully connected to Cadence server at %s", hostPort)
	return cadenceClient, serviceClient, nil
}
