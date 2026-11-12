package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_FromDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "" +
		"HTTP_ADDR=:9090\n" +
		"IRBIS_ADDR=env-host:2222\n" +
		"IRBIS_WORKSTATION=R\n" +
		"AUTH_USERS=librarian|secret|IBIS=read\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	clearEnv(t,
		"HTTP_ADDR", "IRBIS_ADDR", "IRBIS_WORKSTATION",
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
	_ = os.WriteFile(envPath, []byte("IRBIS_ADDR=from-file:6666\nAUTH_USERS=u|p|IBIS=read\n"), 0o600)

	clearEnv(t, "IRBIS_ADDR", "AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE")
	t.Setenv("ENV_FILE", envPath)
	t.Setenv("IRBIS_ADDR", "from-process:7777")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Irbis.Host != "from-process" || cfg.Irbis.Port != 7777 {
		t.Fatalf("host=%q port=%d", cfg.Irbis.Host, cfg.Irbis.Port)
	}
}

func TestParseHostPort(t *testing.T) {
	h, p, err := parseHostPort("192.168.1.4:5555")
	if err != nil || h != "192.168.1.4" || p != 5555 {
		t.Fatalf("%q %d %v", h, p, err)
	}
	h, p, err = parseHostPort("[::1]:6666")
	if err != nil || h != "::1" || p != 6666 {
		t.Fatalf("%q %d %v", h, p, err)
	}
}

func TestLoad_RequiresUsersOrService(t *testing.T) {
	clearEnv(t, "AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE", "IRBIS_WORKSTATION", "IRBIS_ADDR",
		"IRBIS_SERVICE_USERNAME", "IRBIS_SERVICE_PASSWORD")
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_ServiceCredentialsOK(t *testing.T) {
	clearEnv(t, "AUTH_USERS", "AUTH_USERS_FILE", "ENV_FILE",
		"IRBIS_SERVICE_USERNAME", "IRBIS_SERVICE_PASSWORD",
		"IRBIS_INI_PATH", "IRBIS_MENU_PATH", "IRBIS_ACL_CACHE_TTL", "IRBIS_USER_DB_MENU_PATTERN")
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("IRBIS_SERVICE_USERNAME", "admin")
	t.Setenv("IRBIS_SERVICE_PASSWORD", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Irbis.ServiceUsername != "admin" || cfg.Irbis.IniPathCode != "0" || cfg.Irbis.MenuPathCode != "1" {
		t.Fatalf("%+v", cfg.Irbis)
	}
	if cfg.Irbis.ACLCacheTTL != 5*time.Minute {
		t.Fatalf("ttl=%v", cfg.Irbis.ACLCacheTTL)
	}
}

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
