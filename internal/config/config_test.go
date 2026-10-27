package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvAndDotEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
http:
  addr: ":9"
auth:
  users_file: users.yaml
  allow_plaintext: true
irbis:
  host: "yaml-host"
  port: 1111
  workstation: "C"
  use_request_credentials: true
`), 0o600)

	envPath := filepath.Join(dir, ".env")
	_ = os.WriteFile(envPath, []byte("IRBIS_HOST=env-host\nIRBIS_WORKSTATION=R\n"), 0o600)

	t.Setenv("IRBIS_PORT", "2222")
	// Ensure IRBIS_HOST is not set so .env can apply after chdir.
	_ = os.Unsetenv("IRBIS_HOST")
	_ = os.Unsetenv("IRBIS_WORKSTATION")

	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Irbis.Host != "env-host" {
		t.Fatalf("host=%q", cfg.Irbis.Host)
	}
	if cfg.Irbis.Port != 2222 {
		t.Fatalf("port=%d", cfg.Irbis.Port)
	}
	if cfg.Irbis.Workstation != "R" {
		t.Fatalf("ws=%q", cfg.Irbis.Workstation)
	}
}
