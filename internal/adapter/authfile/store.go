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
//
// Example:
//
//	librarian|secret|IBIS=read,write&RDR=read||reader|reader|IBIS=read
//
// bcrypt hashes start with "$2" and are preferred over plaintext.
type Store struct {
	mu             sync.RWMutex
	allowPlaintext bool
	users          map[string]domain.User
}

func New(usersRaw, usersFile string, allowPlaintext bool) (*Store, error) {
	s := &Store{allowPlaintext: allowPlaintext}
	raw := usersRaw
	if usersFile != "" {
		data, err := os.ReadFile(usersFile)
		if err != nil {
			return nil, fmt.Errorf("read users file: %w", err)
		}
		// File: one user per line (same field format). Blank/# lines skipped.
		var parts []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts = append(parts, line)
		}
		raw = strings.Join(parts, "||")
	}
	users, err := parseUsers(raw)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no users configured")
	}
	s.users = users
	return s, nil
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
		parts := strings.SplitN(entry, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("user entry %q: want username|password|databases", entry)
		}
		username := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		dbsRaw := strings.TrimSpace(parts[2])
		if username == "" || secret == "" {
			return nil, fmt.Errorf("user entry %q: empty username/password", entry)
		}
		dbs, err := parseDatabases(dbsRaw)
		if err != nil {
			return nil, fmt.Errorf("user %s: %w", username, err)
		}
		u := domain.User{Username: username, Databases: dbs}
		if strings.HasPrefix(secret, "$2") {
			u.PassHash = secret
		} else {
			u.Password = secret
		}
		users[username] = u
	}
	return users, nil
}

func parseDatabases(raw string) (map[string][]domain.AccessLevel, error) {
	out := make(map[string][]domain.AccessLevel)
	if raw == "" {
		return out, nil
	}
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
		user.Password = ""
		user.PassHash = ""
		return user, nil
	}
	if s.allowPlaintext && user.Password != "" && user.Password == password {
		user.Password = ""
		return user, nil
	}
	return domain.User{}, domain.ErrUnauthorized
}
