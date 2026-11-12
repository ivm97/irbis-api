package authboot

import (
	"fmt"

	"github.com/ivm97/irbis-api/internal/adapter/authfile"
	"github.com/ivm97/irbis-api/internal/adapter/authirbis"
	"github.com/ivm97/irbis-api/internal/config"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

// NewStore picks local AUTH_USERS / AUTH_USERS_FILE (no IRBIS ACL limits) or authirbis.
func NewStore(cfg config.Config, reader authirbis.Reader) (ports.AuthStore, error) {
	if cfg.Auth.Users != "" || cfg.Auth.UsersFile != "" {
		return authfile.New(cfg.Auth.Users, cfg.Auth.UsersFile, cfg.Auth.AllowPlaintext)
	}
	if reader == nil {
		return nil, fmt.Errorf("auth: IRBIS reader required when AUTH_USERS is not set")
	}
	if cfg.Irbis.ServiceUsername == "" || cfg.Irbis.ServicePassword == "" {
		return nil, fmt.Errorf("auth: IRBIS_SERVICE_USERNAME/PASSWORD required")
	}
	return authirbis.New(reader, authirbis.Options{
		Admin: ports.Session{
			Username:    cfg.Irbis.ServiceUsername,
			Password:    cfg.Irbis.ServicePassword,
			Workstation: domain.ARMAdministrator,
		},
		DatabasesMenu:   cfg.Irbis.DatabasesMenu,
		IniPathCode:     cfg.Irbis.IniPathCode,
		MenuPathCode:    cfg.Irbis.MenuPathCode,
		UserMenuPattern: cfg.Irbis.UserDBMenuPattern,
		CacheTTL:        cfg.Irbis.ACLCacheTTL,
	})
}
