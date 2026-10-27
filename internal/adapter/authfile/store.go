package authfile

import (
	"context"
	"fmt"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	"github.com/ivm97/irbis-api/internal/domain"
)

type fileUser struct {
	Username     string              `yaml:"username"`
	Password     string              `yaml:"password"`
	PasswordHash string              `yaml:"password_hash"`
	Databases    map[string][]string `yaml:"databases"`
}

type fileRoot struct {
	Users []fileUser `yaml:"users"`
}

// Store authenticates against a YAML users file.
type Store struct {
	mu             sync.RWMutex
	path           string
	allowPlaintext bool
	users          map[string]domain.User
}

func New(path string, allowPlaintext bool) (*Store, error) {
	s := &Store{path: path, allowPlaintext: allowPlaintext}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Reload() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read users file: %w", err)
	}
	var root fileRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse users file: %w", err)
	}
	users := make(map[string]domain.User, len(root.Users))
	for _, fu := range root.Users {
		if fu.Username == "" {
			continue
		}
		dbs := make(map[string][]domain.AccessLevel, len(fu.Databases))
		for db, levels := range fu.Databases {
			al := make([]domain.AccessLevel, 0, len(levels))
			for _, l := range levels {
				switch l {
				case "read":
					al = append(al, domain.AccessRead)
				case "write":
					al = append(al, domain.AccessWrite)
				}
			}
			dbs[db] = al
		}
		users[fu.Username] = domain.User{
			Username:  fu.Username,
			Password:  fu.Password,
			PassHash:  fu.PasswordHash,
			Databases: dbs,
		}
	}
	s.mu.Lock()
	s.users = users
	s.mu.Unlock()
	return nil
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
