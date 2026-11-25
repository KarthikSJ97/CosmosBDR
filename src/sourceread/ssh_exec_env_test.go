package sourceread

import (
	"strings"
	"testing"
)

func TestPrependEnvToRemoteCommand(t *testing.T) {
	cmd := prependEnvToRemoteCommand(map[string]string{
		"REMOTE_ROOT": "/var/backups",
		"PGPASSWORD":  "s3cret",
	}, "bash -lc 'echo hi'")
	for _, sub := range []string{"REMOTE_ROOT=", "PGPASSWORD=", "bash -lc"} {
		if !strings.Contains(cmd, sub) {
			t.Fatalf("missing %q in %q", sub, cmd)
		}
	}
	if !strings.HasPrefix(cmd, "PGPASSWORD=") {
		t.Fatalf("env keys should be sorted (PGPASSWORD before REMOTE_ROOT): %q", cmd)
	}
}