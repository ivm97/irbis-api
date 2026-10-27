package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ivm97/irbis-api/internal/domain"
)

type Config struct {
	HTTP  HTTPConfig  `yaml:"http"`
	Auth  AuthConfig  `yaml:"auth"`
	Irbis IrbisConfig `yaml:"irbis"`
}

type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

type AuthConfig struct {
	UsersFile      string `yaml:"users_file"`
	AllowPlaintext bool   `yaml:"allow_plaintext"`
}

type IrbisConfig struct {
	Host                  string        `yaml:"host"`
	Port                  int           `yaml:"port"`
	Workstation           string        `yaml:"workstation"`
	DefaultDatabase       string        `yaml:"default_database"`
	DialTimeout           time.Duration `yaml:"dial_timeout"`
	IOTimeout             time.Duration `yaml:"io_timeout"`
	UseRequestCredentials bool          `yaml:"use_request_credentials"`
	ServiceUsername       string        `yaml:"service_username"`
	ServicePassword       string        `yaml:"service_password"`
	DatabasesMenu         string        `yaml:"databases_menu"`
}

// Load reads YAML config, then overlays environment / optional .env file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	_ = loadDotEnv(".env")
	cfg.applyEnv()
	cfg.applyDefaults()
	if _, err := domain.NormalizeWorkstation(cfg.Irbis.Workstation, domain.ARMCataloger); err != nil {
		return Config{}, fmt.Errorf("irbis.workstation: %w", err)
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
}

func (c *Config) applyEnv() {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		c.HTTP.Addr = v
	}
	if v := os.Getenv("AUTH_USERS_FILE"); v != "" {
		c.Auth.UsersFile = v
	}
	if v := os.Getenv("AUTH_ALLOW_PLAINTEXT"); v != "" {
		c.Auth.AllowPlaintext = parseBool(v)
	}
	if v := os.Getenv("IRBIS_HOST"); v != "" {
		c.Irbis.Host = v
	}
	if v := os.Getenv("IRBIS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Irbis.Port = p
		}
	}
	if v := os.Getenv("IRBIS_WORKSTATION"); v != "" {
		c.Irbis.Workstation = v
	}
	if v := os.Getenv("IRBIS_DEFAULT_DATABASE"); v != "" {
		c.Irbis.DefaultDatabase = v
	}
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
	if v := os.Getenv("IRBIS_SERVICE_USERNAME"); v != "" {
		c.Irbis.ServiceUsername = v
	}
	if v := os.Getenv("IRBIS_SERVICE_PASSWORD"); v != "" {
		c.Irbis.ServicePassword = v
	}
	if v := os.Getenv("IRBIS_DATABASES_MENU"); v != "" {
		c.Irbis.DatabasesMenu = v
	}
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
