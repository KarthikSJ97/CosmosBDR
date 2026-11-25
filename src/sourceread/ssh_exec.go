package sourceread

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"cosmosbdr/src/config"

	"golang.org/x/crypto/ssh"
)

// RunSSHStdoutCmd runs a single command string on the remote host (Windows OpenSSH uses cmd.exe; no Unix shell quoting).
func RunSSHStdoutCmd(ctx context.Context, src config.ResolvedSource, cmdLine string) ([]byte, error) {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return nil, fmt.Errorf("ssh exec: empty command")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := connectSSHOnly(src)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmdLine)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return nil, fmt.Errorf("ssh exec failed: %w: %s", err, msg)
			}
			return nil, fmt.Errorf("ssh exec failed: %w", err)
		}
	}
	return stdout.Bytes(), nil
}

// RunSSHStdout runs argv on the remote host and returns stdout (Unix-oriented shell quoting).
func RunSSHStdout(ctx context.Context, src config.ResolvedSource, argv []string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("ssh exec: empty command")
	}
	return RunSSHStdoutCmd(ctx, src, strings.Join(shellQuoteArgs(argv), " "))
}

// RunSSHCommand runs argv on the remote host (remote user's login shell interprets a single quoted command line).
func RunSSHCommand(ctx context.Context, src config.ResolvedSource, argv []string, env map[string]string) error {
	if len(argv) == 0 {
		return fmt.Errorf("ssh exec: empty command")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	client, err := connectSSHOnly(src)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	// Setenv only works when sshd allows AcceptEnv for each key; many hosts do not.
	// Prefix VAR=value on the command so pre_backup scripts see env_refs (bash -lc, set -u).
	for k, v := range env {
		_ = session.Setenv(k, v)
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr
	cmd := prependEnvToRemoteCommand(env, strings.Join(shellQuoteArgs(argv), " "))

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return ctx.Err()
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return fmt.Errorf("ssh exec failed: %w: %s", err, msg)
			}
			return fmt.Errorf("ssh exec failed: %w", err)
		}
	}
	return nil
}

func connectSSHOnly(src config.ResolvedSource) (*ssh.Client, error) {
	cfg, err := sshClientConfig(src)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(src.Host, fmt.Sprintf("%d", src.Port))
	return dialSSH(addr, cfg, src)
}

func shellQuoteArgs(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = shellQuote(a)
	}
	return out
}

func prependEnvToRemoteCommand(env map[string]string, cmd string) string {
	if len(env) == 0 {
		return cmd
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellQuote(env[k]))
		b.WriteByte(' ')
	}
	b.WriteString(cmd)
	return b.String()
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		switch r {
		case 'a', 'z', 'A', 'Z', '0', '9', '_', '-', '.', '/':
			return false
		default:
			return true
		}
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}