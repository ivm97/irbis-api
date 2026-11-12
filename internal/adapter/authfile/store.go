package authfile

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/ivm97/irbis-api/internal/domain"
)

// Store authenticates against users from AUTH_USERS / AUTH_USERS_FILE.
//
// Format (users separated by "||", fields by "|"):
//
//	username|password_or_bcrypt_hash|DB=perm,perm&DB2=perm
//	username|password|*                 — unrestricted (any DB / ARM)
//	username|password|                  — same (empty ACL = unrestricted)
//	username|password                   — same (two fields)
//
// Example:
//
//	librarian|secret|IBIS=read,write&RDR=read||admin|secret|*
//
// bcrypt hashes start with "$2" and are preferred over plaintext.
//
// Local file mode does not apply IRBIS ARM/mnu restrictions: AccessFor ignores ARM.
type Store struct {
	mu             sync.RWMutex
	allowPlaintext bool
	users          map[string]domain.User
}

func New(usersRaw, usersFile string, allowPlaintext bool) (*Store, error) {
	s := &Store{allowPlaintext: allowPlaintext}
	var users map[string]domain.User
	var err error
	if usersFile != "" {
		data, errRead := os.ReadFile(usersFile)
		if errRead != nil {
			return nil, fmt.Errorf("read users file: %w", errRead)
		}
		users, err = parseUsersFile(string(data))
	} else {
		users, err = parseUsers(usersRaw)
	}
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no users configured")
	}
	s.users = users
	return s, nil
}

// parseUsersFile: one user per line; blank/# lines skipped. Does not join via "||"
// (a trailing "|" on a line would break that encoding).
func parseUsersFile(data string) (map[string]domain.User, error) {
	users := make(map[string]domain.User)
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := parseUserEntry(line)
		if err != nil {
			return nil, err
		}
		users[u.Username] = u
	}
	return users, nil
}

func parseUsers(raw string) (map[string]domain.User, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	users := make(map[string]domain.User)
	for _, entry := range strings.Split(raw, "||") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		u, err := parseUserEntry(entry)
		if err != nil {
			return nil, err
		}
		users[u.Username] = u
	}
	return users, nil
}

func parseUserEntry(entry string) (domain.User, error) {
	parts := strings.SplitN(entry, "|", 3)
	if len(parts) < 2 {
		return domain.User{}, fmt.Errorf("user entry %q: want username|password[|databases]", entry)
	}
	username := strings.TrimSpace(parts[0])
	secret := strings.TrimSpace(parts[1])
	dbsRaw := ""
	if len(parts) == 3 {
		dbsRaw = strings.TrimSpace(parts[2])
	}
	if username == "" || secret == "" {
		return domain.User{}, fmt.Errorf("user entry %q: empty username/password", entry)
	}
	u := domain.User{Username: username}
	if dbsRaw == "" || dbsRaw == "*" {
		u.Unrestricted = true
	} else {
		dbs, err := parseDatabases(dbsRaw)
		if err != nil {
			return domain.User{}, fmt.Errorf("user %s: %w", username, err)
		}
		u.Databases = dbs
	}
	if strings.HasPrefix(secret, "$2") {
		u.PassHash = secret
	} else {
		u.Password = secret
	}
	return u, nil
}

func parseDatabases(raw string) (map[string][]domain.AccessLevel, error) {
	out := make(map[string][]domain.AccessLevel)
	for _, chunk := range strings.Split(raw, "&") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		name, perms, ok := strings.Cut(chunk, "=")
		if !ok {
			return nil, fmt.Errorf("database entry %q: want DB=read,write", chunk)
		}
		name = strings.TrimSpace(name)
		al := make([]domain.AccessLevel, 0, 2)
		for _, p := range strings.Split(perms, ",") {
			switch strings.ToLower(strings.TrimSpace(p)) {
			case "read":
				al = append(al, domain.AccessRead)
			case "write":
				al = append(al, domain.AccessWrite)
			case "":
				continue
			default:
				return nil, fmt.Errorf("unknown permission %q for %s", p, name)
			}
		}
		if name == "" || len(al) == 0 {
			return nil, fmt.Errorf("invalid database entry %q", chunk)
		}
		out[name] = al
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty database list")
	}
	return out, nil
}

func (s *Store) Authenticate(_ context.Context, username, password string) (domain.User, error) {
	if username == "" || password == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	s.mu.RLock()
	user, ok := s.users[username]
	s.mu.RUnlock()
	if !ok {
		return domain.User{}, domain.ErrUnauthorized
	}
	if user.PassHash != "" {
		if bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password)) != nil {
			return domain.User{}, domain.ErrUnauthorized
		}
		return publicUser(user), nil
	}
	if s.allowPlaintext && user.Password != "" && user.Password == password {
		return publicUser(user), nil
	}
	return domain.User{}, domain.ErrUnauthorized
}

func publicUser(user domain.User) domain.User {
	out := user
	out.Password = ""
	out.PassHash = ""
	if out.Unrestricted {
		out.Databases = nil
	} else if user.Databases != nil {
		out.Databases = cloneDBMap(user.Databases)
	}
	return out
}

// AccessFor returns ACL from the local file. ARM is ignored (no IRBIS restrictions).
func (s *Store) AccessFor(_ context.Context, username string, _ domain.Workstation) (domain.DBAccess, error) {
	s.mu.RLock()
	user, ok := s.users[username]
	s.mu.RUnlock()
	if !ok {
		return domain.DBAccess{}, domain.ErrUnauthorized
	}
	if user.Unrestricted {
		return domain.DBAccess{Unrestricted: true}, nil
	}
	return domain.DBAccess{Databases: cloneDBMap(user.Databases)}, nil
}

func cloneDBMap(in map[string][]domain.AccessLevel) map[string][]domain.AccessLevel {
	out := make(map[string][]domain.AccessLevel, len(in))
	for db, levels := range in {
		cp := make([]domain.AccessLevel, len(levels))
		copy(cp, levels)
		out[db] = cp
	}
	return out
}
