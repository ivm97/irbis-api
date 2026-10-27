package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FromDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "" +
		"HTTP_ADDR=:9090\n" +
		"IRBIS_HOST=env-host\n" +
		"IRBIS_PORT=2222\n" +
		"IRBIS_WORKSTATION=R\n" +
		"AUTH_USERS=librarian|secret|IBIS=read\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	clearEnv(t,
		"HTTP_ADDR", "IRBIS_HOST", "IRBIS_PORT", "IRBIS_WORKSTATION",
		"AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE",
		"IRBIS_USE_REQUEST_CREDENTIALS",
	)
	t.Setenv("ENV_FILE", envPath)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != ":9090" || cfg.Irbis.Host != "env-host" || cfg.Irbis.Port != 2222 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Irbis.Workstation != "R" {
		t.Fatalf("ws=%q", cfg.Irbis.Workstation)
	}
	if cfg.Auth.Users == "" {
		t.Fatal("users empty")
	}
	if !cfg.Irbis.UseRequestCredentials {
		t.Fatal("default use_request_credentials should be true")
	}
}

func TestLoad_ProcessEnvWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	_ = os.WriteFile(envPath, []byte("IRBIS_HOST=from-file\nAUTH_USERS=u|p|IBIS=read\n"), 0o600)

	clearEnv(t, "IRBIS_HOST", "AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE")
	t.Setenv("ENV_FILE", envPath)
	t.Setenv("IRBIS_HOST", "from-process")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Irbis.Host != "from-process" {
		t.Fatalf("host=%q", cfg.Irbis.Host)
	}
}

func TestLoad_RequiresUsers(t *testing.T) {
	clearEnv(t, "AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE", "IRBIS_WORKSTATION")
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
