package authirbis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/ivm97/irbis-api/internal/adapter/irbisgw"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

// Reader loads client_m / ini / mnu via an admin IRBIS session.
type Reader interface {
	GetUserList(ctx context.Context, sess ports.Session) ([]irbisgw.IrbisUserInfo, error)
	ReadIniDBNameCat(ctx context.Context, sess ports.Session, specification string) (string, error)
	ReadDatabaseMenu(ctx context.Context, sess ports.Session, specification string) ([]domain.DatabaseInfo, error)
}

// Options configures lazy ACL auth against IRBIS server files.
type Options struct {
	Admin           ports.Session
	DatabasesMenu   string // fallback menu spec, e.g. 1..dbnam2.mnu
	IniPathCode     string // "0"
	MenuPathCode    string // "1"
	UserMenuPattern string // optional db_{user}.mnu
	CacheTTL        time.Duration
}

// Store authenticates against client_m and resolves DB ACL from user mnu on demand.
type Store struct {
	reader Reader
	admin  ports.Session

	databasesMenu   string
	iniPathCode     string
	menuPathCode    string
	userMenuPattern string
	ttl             time.Duration

	usersSF singleflight.Group
	aclSF   singleflight.Group

	mu        sync.RWMutex
	users     map[string]irbisgw.IrbisUserInfo // lower(name) → entry
	usersAt   time.Time
	acl       map[string]aclEntry // key: user\x00ARM
}

type aclEntry struct {
	at     time.Time
	access domain.DBAccess
	deny   bool // ARM not allowed
}

func New(reader Reader, opt Options) (*Store, error) {
	if reader == nil {
		return nil, fmt.Errorf("authirbis: reader required")
	}
	if opt.Admin.Username == "" || opt.Admin.Password == "" {
		return nil, fmt.Errorf("authirbis: admin credentials required")
	}
	if opt.Admin.Workstation == "" {
		opt.Admin.Workstation = domain.ARMAdministrator
	}
	if opt.DatabasesMenu == "" {
		opt.DatabasesMenu = "1..dbnam2.mnu"
	}
	if opt.IniPathCode == "" {
		opt.IniPathCode = "0"
	}
	if opt.MenuPathCode == "" {
		opt.MenuPathCode = "1"
	}
	if opt.CacheTTL <= 0 {
		opt.CacheTTL = 5 * time.Minute
	}
	return &Store{
		reader:          reader,
		admin:           opt.Admin,
		databasesMenu:   opt.DatabasesMenu,
		iniPathCode:     opt.IniPathCode,
		menuPathCode:    opt.MenuPathCode,
		userMenuPattern: opt.UserMenuPattern,
		ttl:             opt.CacheTTL,
		users:           make(map[string]irbisgw.IrbisUserInfo),
		acl:             make(map[string]aclEntry),
	}, nil
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	if username == "" || password == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	users, err := s.loadUsers(ctx)
	if err != nil {
		return domain.User{}, err
	}
	u, ok := users[strings.ToLower(username)]
	if !ok || u.Password != password {
		return domain.User{}, domain.ErrUnauthorized
	}
	return domain.User{Username: u.Name}, nil
}

// AccessFor resolves DB rights for (username, ARM). Cache miss reads ini + mnu via admin.
func (s *Store) AccessFor(ctx context.Context, username string, arm domain.Workstation) (domain.DBAccess, error) {
	if username == "" {
		return domain.DBAccess{}, domain.ErrUnauthorized
	}
	if arm == "" {
		arm = domain.ARMCataloger
	}

	key := aclKey(username, arm)
	if access, deny, ok := s.getACL(key); ok {
		if deny {
			return domain.DBAccess{}, fmt.Errorf("%w: workstation %s not allowed", domain.ErrForbidden, arm)
		}
		return cloneAccess(access), nil
	}

	v, err, _ := s.aclSF.Do(key, func() (any, error) {
		if access, deny, ok := s.getACL(key); ok {
			if deny {
				return nil, fmt.Errorf("%w: workstation %s not allowed", domain.ErrForbidden, arm)
			}
			return access, nil
		}
		access, deny, err := s.resolveACL(ctx, username, arm)
		if err != nil {
			return nil, err
		}
		s.putACL(key, access, deny)
		if deny {
			return nil, fmt.Errorf("%w: workstation %s not allowed", domain.ErrForbidden, arm)
		}
		return access, nil
	})
	if err != nil {
		return domain.DBAccess{}, err
	}
	return cloneAccess(v.(domain.DBAccess)), nil
}

func (s *Store) loadUsers(ctx context.Context) (map[string]irbisgw.IrbisUserInfo, error) {
	s.mu.RLock()
	if len(s.users) > 0 && time.Since(s.usersAt) < s.ttl {
		out := s.users
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	v, err, _ := s.usersSF.Do("users", func() (any, error) {
		s.mu.RLock()
		if len(s.users) > 0 && time.Since(s.usersAt) < s.ttl {
			out := s.users
			s.mu.RUnlock()
			return out, nil
		}
		s.mu.RUnlock()

		list, err := s.reader.GetUserList(ctx, s.admin)
		if err != nil {
			return nil, err
		}
		next := make(map[string]irbisgw.IrbisUserInfo, len(list))
		for _, u := range list {
			name := strings.TrimSpace(u.Name)
			if name == "" {
				continue
			}
			next[strings.ToLower(name)] = u
		}
		s.mu.Lock()
		s.users = next
		s.usersAt = time.Now()
		s.mu.Unlock()
		return next, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]irbisgw.IrbisUserInfo), nil
}

func (s *Store) resolveACL(ctx context.Context, username string, arm domain.Workstation) (domain.DBAccess, bool, error) {
	users, err := s.loadUsers(ctx)
	if err != nil {
		return domain.DBAccess{}, false, err
	}
	u, ok := users[strings.ToLower(username)]
	if !ok {
		return domain.DBAccess{}, false, domain.ErrUnauthorized
	}
	iniName, ok := u.ArmIni[arm]
	if !ok || strings.TrimSpace(iniName) == "" {
		return domain.DBAccess{}, true, nil // deny ARM
	}

	menuSpec, err := s.resolveMenuSpec(ctx, username, iniName)
	if err != nil {
		return domain.DBAccess{}, false, err
	}
	dbs, err := s.reader.ReadDatabaseMenu(ctx, s.admin, menuSpec)
	if err != nil {
		return domain.DBAccess{}, false, err
	}
	if len(dbs) == 0 && menuSpec != s.databasesMenu {
		// personal/empty → fallback to ARM-wide list (dbnam2 = «все базы» АРМа)
		dbs, err = s.reader.ReadDatabaseMenu(ctx, s.admin, s.databasesMenu)
		if err != nil {
			return domain.DBAccess{}, false, err
		}
	}
	return domain.DBAccess{Databases: menuToAccess(dbs)}, false, nil
}

func (s *Store) resolveMenuSpec(ctx context.Context, username, iniName string) (string, error) {
	iniSpec := fileSpec(s.iniPathCode, iniName)
	dbn, err := s.reader.ReadIniDBNameCat(ctx, s.admin, iniSpec)
	if err != nil {
		return "", err
	}
	dbn = strings.TrimSpace(dbn)
	if dbn != "" {
		return s.menuFileSpec(dbn), nil
	}
	if s.userMenuPattern != "" {
		name := expandUserPattern(s.userMenuPattern, username)
		if name != "" {
			return fileSpec(s.menuPathCode, name), nil
		}
	}
	return s.databasesMenu, nil
}

func (s *Store) menuFileSpec(nameOrSpec string) string {
	nameOrSpec = strings.TrimSpace(nameOrSpec)
	if nameOrSpec == "" {
		return s.databasesMenu
	}
	// Already a full IRBIS specification (path.db.file)?
	if strings.Contains(nameOrSpec, "..") || strings.Count(nameOrSpec, ".") >= 2 {
		return nameOrSpec
	}
	return fileSpec(s.menuPathCode, nameOrSpec)
}

func fileSpec(pathCode, filename string) string {
	filename = strings.TrimSpace(filename)
	return pathCode + ".." + filename
}

func expandUserPattern(pattern, username string) string {
	u := strings.TrimSpace(username)
	p := strings.ReplaceAll(pattern, "{user}", u)
	p = strings.ReplaceAll(p, "{USER}", strings.ToUpper(u))
	return strings.TrimSpace(p)
}

func menuToAccess(dbs []domain.DatabaseInfo) map[string][]domain.AccessLevel {
	out := make(map[string][]domain.AccessLevel, len(dbs))
	for _, db := range dbs {
		name := strings.TrimSpace(db.Name)
		if name == "" {
			continue
		}
		if db.ReadOnly {
			out[name] = []domain.AccessLevel{domain.AccessRead}
		} else {
			out[name] = []domain.AccessLevel{domain.AccessRead, domain.AccessWrite}
		}
	}
	return out
}

func aclKey(username string, arm domain.Workstation) string {
	return strings.ToLower(strings.TrimSpace(username)) + "\x00" + string(arm)
}

func (s *Store) getACL(key string) (domain.DBAccess, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.acl[key]
	if !ok || time.Since(e.at) >= s.ttl {
		return domain.DBAccess{}, false, false
	}
	return e.access, e.deny, true
}

func (s *Store) putACL(key string, access domain.DBAccess, deny bool) {
	s.mu.Lock()
	s.acl[key] = aclEntry{at: time.Now(), access: access, deny: deny}
	s.mu.Unlock()
}

func cloneAccess(in domain.DBAccess) domain.DBAccess {
	if in.Unrestricted {
		return domain.DBAccess{Unrestricted: true}
	}
	out := domain.DBAccess{Databases: make(map[string][]domain.AccessLevel, len(in.Databases))}
	for k, v := range in.Databases {
		cp := make([]domain.AccessLevel, len(v))
		copy(cp, v)
		out.Databases[k] = cp
	}
	return out
}
