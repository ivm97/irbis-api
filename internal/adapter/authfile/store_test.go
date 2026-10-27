package authfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivm97/irbis-api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestStore_FromENVString(t *testing.T) {
	store, err := New(
		"librarian|secret|IBIS=read,write&RDR=read||reader|reader|IBIS=read",
		"",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.Authenticate(context.Background(), "librarian", "secret")
	if err != nil || !u.Can("IBIS", domain.AccessWrite) || !u.Can("RDR", domain.AccessRead) {
		t.Fatalf("%v %+v", err, u)
	}
	_, err = store.Authenticate(context.Background(), "librarian", "wrong")
	if err != domain.ErrUnauthorized {
		t.Fatalf("%v", err)
	}
}

func TestStore_BcryptAndFile(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("hashed"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	content := "# comment\n" +
		"hashed|" + string(hash) + "|IBIS=read,write\n" +
		"plain|secret|IBIS=read\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := New("", path, true)
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.Authenticate(context.Background(), "hashed", "hashed")
	if err != nil || !u.Can("IBIS", domain.AccessWrite) {
		t.Fatalf("%v %+v", err, u)
	}
	_, err = store.Authenticate(context.Background(), "plain", "secret")
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_NoPlaintext(t *testing.T) {
	store, err := New("u|p|IBIS=read", "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Authenticate(context.Background(), "u", "p")
	if err != domain.ErrUnauthorized {
		t.Fatalf("%v", err)
	}
}

func TestParseUsers_BadEntry(t *testing.T) {
	_, err := New("broken", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
}
