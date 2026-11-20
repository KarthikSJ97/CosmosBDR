package config

import "testing"

func TestEnvPrefixFromSourceID(t *testing.T) {
	if got := EnvPrefixFromSourceID("win-home-laptop"); got != "WIN_HOME_LAPTOP" {
		t.Fatalf("got %q", got)
	}
	if got := EnvPrefixFromSourceID("prod-app-01"); got != "PROD_APP_01" {
		t.Fatalf("got %q", got)
	}
}