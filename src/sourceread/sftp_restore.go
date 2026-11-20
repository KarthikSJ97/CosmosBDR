package sourceread

import (
	"context"
	"fmt"
	"os"
	"path"

	"cosmosbdr/src/config"
)

// PrepareSFTPRestoreFile creates parent directories if needed, then creates or truncates
// the remote file to originalSize (overwrite when the path already exists).
func PrepareSFTPRestoreFile(ctx context.Context, src config.ResolvedSource, remoteObjectPath string, originalSize int64) error {
	_ = ctx
	if originalSize < 0 {
		return fmt.Errorf("original_size cannot be negative")
	}

	sshClient, sftpClient, err := connectSFTP(src)
	if err != nil {
		return err
	}
	defer sshClient.Close()
	defer sftpClient.Close()

	remoteObjectPath = path.Clean(remoteObjectPath)
	parentDir := path.Dir(remoteObjectPath)
	if parentDir != "." && parentDir != "/" {
		if err := sftpClient.MkdirAll(parentDir); err != nil {
			return fmt.Errorf("sftp mkdir %q: %w", parentDir, err)
		}
	}

	f, err := sftpClient.OpenFile(remoteObjectPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sftp create %q: %w", remoteObjectPath, err)
	}

	if originalSize > 0 {
		if err := f.Truncate(originalSize); err != nil {
			f.Close()
			return fmt.Errorf("sftp truncate %q: %w", remoteObjectPath, err)
		}
	}

	return f.Close()
}

// WriteSFTPAt writes a segment to an existing remote file at offset.
func WriteSFTPAt(ctx context.Context, src config.ResolvedSource, remoteObjectPath string, offset int64, data []byte) (int, error) {
	_ = ctx

	sshClient, sftpClient, err := connectSFTP(src)
	if err != nil {
		return 0, err
	}
	defer sshClient.Close()
	defer sftpClient.Close()

	remoteObjectPath = path.Clean(remoteObjectPath)
	f, err := sftpClient.OpenFile(remoteObjectPath, os.O_RDWR)
	if err != nil {
		return 0, fmt.Errorf("sftp open %q for write: %w", remoteObjectPath, err)
	}
	defer f.Close()

	n, err := f.WriteAt(data, offset)
	if err != nil {
		return n, fmt.Errorf("sftp write at %d: %w", offset, err)
	}
	return n, nil
}