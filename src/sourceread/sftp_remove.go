package sourceread

import (
	"context"
	"fmt"
	"os"

	"cosmosbdr/src/config"
)

// RemoveSFTPFile deletes a single remote file; missing files are ignored.
func RemoveSFTPFile(ctx context.Context, src config.ResolvedSource, remotePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sshClient, sftpClient, err := connectSFTP(src)
	if err != nil {
		return err
	}
	defer sshClient.Close()
	defer sftpClient.Close()

	if err := sftpClient.Remove(remotePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("sftp remove %q: %w", remotePath, err)
	}
	return nil
}