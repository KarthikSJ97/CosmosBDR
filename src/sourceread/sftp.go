package sourceread

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"path"
	"strings"
	"time"

	"cosmosbdr/src/config"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPFileReader streams a remote file over SFTP.
type SFTPFileReader struct {
	remotePath string
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	file       *sftp.File
}

func NewSFTPFileReader(ctx context.Context, src config.ResolvedSource, remoteObjectPath string) (*SFTPFileReader, error) {
	_ = ctx
	client, scl, err := connectSFTP(src)
	if err != nil {
		return nil, err
	}

	remoteObjectPath = path.Clean(remoteObjectPath)
	f, err := scl.Open(remoteObjectPath)
	if err != nil {
		scl.Close()
		client.Close()
		return nil, fmt.Errorf("sftp open %q: %w", remoteObjectPath, err)
	}

	return &SFTPFileReader{
		remotePath: remoteObjectPath,
		sshClient:  client,
		sftpClient: scl,
		file:       f,
	}, nil
}

func dialSSH(addr string, cfg *ssh.ClientConfig, src config.ResolvedSource) (*ssh.Client, error) {
	if strings.TrimSpace(src.SSH.ProxyJump) == "" {
		return ssh.Dial("tcp", addr, cfg)
	}
	// Bastion hop: proxy jump host uses same credentials (dev-friendly; refine per-source later).
	jumpAddr := net.JoinHostPort(src.SSH.ProxyJump, fmt.Sprintf("%d", src.Port))
	jump, err := ssh.Dial("tcp", jumpAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("proxy_jump %s: %w", jumpAddr, err)
	}
	conn, err := jump.Dial("tcp", addr)
	if err != nil {
		jump.Close()
		return nil, fmt.Errorf("forward to %s: %w", addr, err)
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		jump.Close()
		return nil, err
	}
	client := ssh.NewClient(ncc, chans, reqs)
	return client, nil
}

func sshClientConfig(src config.ResolvedSource) (*ssh.ClientConfig, error) {
	auth, err := sshAuthMethods(src)
	if err != nil {
		return nil, err
	}

	var hostKeyCallback ssh.HostKeyCallback
	switch strings.ToLower(strings.TrimSpace(src.SSH.KnownHostsPolicy)) {
	case "", "pinned":
		fp := strings.TrimSpace(src.SSH.HostKeyFingerprint)
		if fp == "" {
			return nil, fmt.Errorf("source %q: host_key_fingerprint is required for ssh", src.ID)
		}
		hostKeyCallback = pinnedHostKeyCallback(fp)
	default:
		return nil, fmt.Errorf("source %q: unsupported known_hosts_policy %q", src.ID, src.SSH.KnownHostsPolicy)
	}

	return &ssh.ClientConfig{
		User:            src.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		// Prefer Ed25519 so pinned host_key_fingerprint matches what admins record from ssh-keyscan.
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoED25519,
			ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoRSA,
		},
		Timeout: 30 * time.Second,
	}, nil
}

func sshAuthMethods(src config.ResolvedSource) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if pass := strings.TrimSpace(src.ResolvedSSHPassword); pass != "" {
		methods = append(methods, ssh.Password(pass))
	}
	if pem := strings.TrimSpace(src.ResolvedSecret); pem != "" {
		signer, err := ssh.ParsePrivateKey([]byte(pem))
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("source %q: no ssh credentials configured", src.ID)
	}
	return methods, nil
}

func pinnedHostKeyCallback(expected string) ssh.HostKeyCallback {
	expected = normalizeFingerprint(expected)
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		got := normalizeFingerprint(ssh.FingerprintSHA256(key))
		if got != expected {
			return fmt.Errorf("ssh host key mismatch: got %s want %s", got, expected)
		}
		return nil
	}
}

func normalizeFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	fp = strings.TrimPrefix(fp, "SHA256:")
	return fp
}

func (r *SFTPFileReader) Stat(ctx context.Context) (FileStat, error) {
	_ = ctx
	info, err := r.sftpClient.Stat(r.remotePath)
	if err != nil {
		return FileStat{}, err
	}
	return FileStat{
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

func (r *SFTPFileReader) ReadAt(ctx context.Context, offset int64, length int) ([]byte, error) {
	_ = ctx
	buf := make([]byte, length)
	n, err := r.file.ReadAt(buf, offset)
	if n > 0 {
		return buf[:n], nil
	}
	if err == nil {
		return buf[:0], nil
	}
	return nil, fmt.Errorf("sftp read at %d: %w", offset, err)
}

func (r *SFTPFileReader) ContentSHA256(ctx context.Context) (string, error) {
	_ = ctx
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("sftp seek start: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, r.file); err != nil {
		return "", fmt.Errorf("sftp hash read: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *SFTPFileReader) Close() error {
	var first error
	if r.file != nil {
		if err := r.file.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.sftpClient != nil {
		if err := r.sftpClient.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.sshClient != nil {
		if err := r.sshClient.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}