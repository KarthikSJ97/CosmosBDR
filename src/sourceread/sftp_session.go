package sourceread

import (
	"fmt"
	"net"

	"cosmosbdr/src/config"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func connectSFTP(src config.ResolvedSource) (*ssh.Client, *sftp.Client, error) {
	cfg, err := sshClientConfig(src)
	if err != nil {
		return nil, nil, err
	}

	addr := net.JoinHostPort(src.Host, fmt.Sprintf("%d", src.Port))
	client, err := dialSSH(addr, cfg, src)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	scl, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("sftp client: %w", err)
	}

	return client, scl, nil
}