package authboot

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivm97/irbis-api/internal/adapter/authfile"
	"github.com/ivm97/irbis-api/internal/adapter/authirbis"
	"github.com/ivm97/irbis-api/internal/adapter/irbisgw"
	"github.com/ivm97/irbis-api/internal/config"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

type stubReader struct{}

func (stubReader) GetUserList(context.Context, ports.Session) ([]irbisgw.IrbisUserInfo, error) {
	return []irbisgw.IrbisUserInfo{{
		Name: "svc", Password: "x",
		ArmIni: map[domain.Workstation]string{domain.ARMCataloger: "irbisc.ini"},
	}}, nil
}
func (stubReader) ReadIniDBNameCat(context.Context, ports.Session, string) (string, error) {
	return "", nil
}
func (stubReader) ReadDatabaseMenu(context.Context, ports.Session, string) ([]domain.DatabaseInfo, error) {
	return []domain.DatabaseInfo{{Name: "IBIS"}}, nil
}

func TestNewStore_LocalFileUnrestricted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	if err := os.WriteFile(path, []byte("admin|secret|*\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Auth: config.AuthConfig{UsersFile: path, AllowPlaintext: true},
	}
	store, err := NewStore(cfg, nil) // reader unused for file mode
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.(*authfile.Store); !ok {
		t.Fatalf("%T", store)
	}
	u, err := store.Authenticate(context.Background(), "admin", "secret")
	if err != nil || !u.Unrestricted {
		t.Fatalf("%v %+v", err, u)
	}
	access, err := store.AccessFor(context.Background(), "admin", domain.ARMReader)
	if err != nil || !access.Unrestricted {
		t.Fatalf("file mode must ignore ARM: %v %+v", err, access)
	}
}

func TestNewStore_LocalUsersString(t *testing.T) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			Users:          "u|p|IBIS=read",
			AllowPlaintext: true,
		},
		Irbis: config.IrbisConfig{ServiceUsername: "admin", ServicePassword: "x"},
	}
	store, err := NewStore(cfg, stubReader{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.(*authfile.Store); !ok {
		t.Fatal("AUTH_USERS must win over service")
	}
}

func TestNewStore_IrbisACL(t *testing.T) {
	cfg := config.Config{
		Irbis: config.IrbisConfig{
			ServiceUsername: "admin",
			ServicePassword: "secret",
			DatabasesMenu:   "1..dbnam2.mnu",
			IniPathCode:     "0",
			MenuPathCode:    "1",
		},
	}
	store, err := NewStore(cfg, stubReader{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.(*authirbis.Store); !ok {
		t.Fatalf("%T", store)
	}
}

func TestNewStore_RequiresReaderOrUsers(t *testing.T) {
	cfg := config.Config{
		Irbis: config.IrbisConfig{ServiceUsername: "a", ServicePassword: "b"},
	}
	if _, err := NewStore(cfg, nil); err == nil {
		t.Fatal("expected error")
	}
	cfg = config.Config{}
	if _, err := NewStore(cfg, stubReader{}); err == nil {
		t.Fatal("expected error")
	}
}
