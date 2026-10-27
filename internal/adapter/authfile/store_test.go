package authfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivm97/irbis-api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestStore_PlainAndHash(t *testing.T) {
	dir := t.TempDir()
	hash, err := bcrypt.GenerateFromPassword([]byte("hashed"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "users.yaml")
	content := "users:\n" +
		"  - username: plain\n" +
		"    password: secret\n" +
		"    databases:\n" +
		"      IBIS: [read]\n" +
		"  - username: hashed\n" +
		"    password_hash: \"" + string(hash) + "\"\n" +
		"    databases:\n" +
		"      IBIS: [read, write]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := New(path, true)
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.Authenticate(context.Background(), "plain", "secret")
	if err != nil || u.Username != "plain" {
		t.Fatalf("%v %+v", err, u)
	}
	u, err = store.Authenticate(context.Background(), "hashed", "hashed")
	if err != nil || !u.Can("IBIS", domain.AccessWrite) {
		t.Fatalf("%v %+v", err, u)
	}
	_, err = store.Authenticate(context.Background(), "plain", "wrong")
	if err != domain.ErrUnauthorized {
		t.Fatalf("%v", err)
	}
}

func TestStore_NoPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.yaml")
	_ = os.WriteFile(path, []byte("users:\n  - username: u\n    password: p\n    databases: {IBIS: [read]}\n"), 0o600)
	store, err := New(path, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Authenticate(context.Background(), "u", "p")
	if err != domain.ErrUnauthorized {
		t.Fatalf("%v", err)
	}
}
