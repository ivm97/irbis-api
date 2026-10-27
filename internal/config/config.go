package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivm97/irbis-api/internal/domain"
)

type Config struct {
	HTTP     HTTPConfig
	Auth     AuthConfig
	Irbis    IrbisConfig
	PrintDir string // TEMPLATES_DIR; default ./templates
}

type HTTPConfig struct {
	Addr           string
	CORSAllowOrigin string // e.g. http://localhost:5173; empty = no CORS middleware
}

type AuthConfig struct {
	// Users is the raw AUTH_USERS value (see ParseUsers).
	Users          string
	UsersFile      string
	AllowPlaintext bool
}

type IrbisConfig struct {
	Host                  string
	Port                  int
	Workstation           string
	DefaultDatabase       string
	DialTimeout           time.Duration
	IOTimeout             time.Duration
	UseRequestCredentials bool
	ServiceUsername       string
	ServicePassword       string
	DatabasesMenu         string
}

// Load reads optional .env (ENV_FILE or .env), then builds config from process env.
func Load() (Config, error) {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	_ = loadDotEnv(envFile)

	cfg := Config{}
	cfg.applyEnv()
	cfg.applyDefaults()

	if _, err := domain.NormalizeWorkstation(cfg.Irbis.Workstation, domain.ARMCataloger); err != nil {
		return Config{}, fmt.Errorf("IRBIS_WORKSTATION: %w", err)
	}
	if cfg.Auth.Users == "" && cfg.Auth.UsersFile == "" {
		return Config{}, fmt.Errorf("set AUTH_USERS or AUTH_USERS_FILE")
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.HTTP.Addr == "" {
		c.HTTP.Addr = ":8080"
	}
	if c.Irbis.Host == "" {
		c.Irbis.Host = "127.0.0.1"
	}
	if c.Irbis.Port == 0 {
		c.Irbis.Port = 6666
	}
	if c.Irbis.Workstation == "" {
		c.Irbis.Workstation = string(domain.ARMCataloger)
	}
	if c.Irbis.DialTimeout == 0 {
		c.Irbis.DialTimeout = 10 * time.Second
	}
	if c.Irbis.DatabasesMenu == "" {
		c.Irbis.DatabasesMenu = "1..dbnam2.mnu"
	}
	if c.PrintDir == "" {
		c.PrintDir = "templates"
	}
	// Default: request credentials go to IRBIS unless explicitly disabled.
	if os.Getenv("IRBIS_USE_REQUEST_CREDENTIALS") == "" {
		c.Irbis.UseRequestCredentials = true
	}
}

func (c *Config) applyEnv() {
	c.HTTP.Addr = os.Getenv("HTTP_ADDR")
	c.HTTP.CORSAllowOrigin = os.Getenv("CORS_ALLOW_ORIGIN")
	c.Auth.Users = os.Getenv("AUTH_USERS")
	c.Auth.UsersFile = os.Getenv("AUTH_USERS_FILE")
	if v := os.Getenv("AUTH_ALLOW_PLAINTEXT"); v != "" {
		c.Auth.AllowPlaintext = parseBool(v)
	} else {
		c.Auth.AllowPlaintext = true
	}

	if addr := strings.TrimSpace(os.Getenv("IRBIS_ADDR")); addr != "" {
		host, port, err := parseHostPort(addr)
		if err == nil {
			c.Irbis.Host = host
			c.Irbis.Port = port
		}
	}
	c.Irbis.Workstation = os.Getenv("IRBIS_WORKSTATION")
	c.Irbis.DefaultDatabase = os.Getenv("IRBIS_DEFAULT_DATABASE")
	if v := os.Getenv("IRBIS_DIAL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Irbis.DialTimeout = d
		}
	}
	if v := os.Getenv("IRBIS_IO_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Irbis.IOTimeout = d
		}
	}
	if v := os.Getenv("IRBIS_USE_REQUEST_CREDENTIALS"); v != "" {
		c.Irbis.UseRequestCredentials = parseBool(v)
	}
	c.Irbis.ServiceUsername = os.Getenv("IRBIS_SERVICE_USERNAME")
	c.Irbis.ServicePassword = os.Getenv("IRBIS_SERVICE_PASSWORD")
	c.Irbis.DatabasesMenu = os.Getenv("IRBIS_DATABASES_MENU")
	c.PrintDir = os.Getenv("TEMPLATES_DIR")
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// parseHostPort accepts "host:port" (also "[::1]:6666").
func parseHostPort(addr string) (host string, port int, err error) {
	addr = strings.TrimSpace(addr)
	// Allow host without port — defaults applied later.
	if !strings.Contains(addr, ":") {
		return addr, 0, nil
	}
	h, p, err := splitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(p)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port in IRBIS_ADDR %q", addr)
	}
	return h, port, nil
}

func splitHostPort(addr string) (host, port string, err error) {
	// Bracketed IPv6: [::1]:6666
	if strings.HasPrefix(addr, "[") {
		end := strings.IndexByte(addr, ']')
		if end < 0 || end+1 >= len(addr) || addr[end+1] != ':' {
			return "", "", fmt.Errorf("invalid IRBIS_ADDR %q", addr)
		}
		return addr[1:end], addr[end+2:], nil
	}
	i := strings.LastIndexByte(addr, ':')
	if i <= 0 || i == len(addr)-1 {
		return "", "", fmt.Errorf("invalid IRBIS_ADDR %q", addr)
	}
	return addr[:i], addr[i+1:], nil
}

// loadDotEnv loads KEY=VALUE lines into the process env if the file exists.
// Existing environment variables are not overwritten.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
