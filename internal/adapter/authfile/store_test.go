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
		"plain|secret|IBIS=read\n" +
		"\n" +
		"boss|boss|*\n" +
		"root|root|\n" +
		"twofields|pw\n"
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

	for _, name := range []string{"boss", "root", "twofields"} {
		pass := map[string]string{"boss": "boss", "root": "root", "twofields": "pw"}[name]
		u, err = store.Authenticate(context.Background(), name, pass)
		if err != nil || !u.Unrestricted {
			t.Fatalf("%s: %v %+v", name, err, u)
		}
		if !u.Can("ANYDB", domain.AccessWrite) {
			t.Fatalf("%s should be unrestricted", name)
		}
		access, err := store.AccessFor(context.Background(), name, domain.ARMAdministrator)
		if err != nil || !access.Unrestricted {
			t.Fatalf("%s AccessFor ARM A: %v %+v", name, err, access)
		}
		access, err = store.AccessFor(context.Background(), name, domain.ARMReader)
		if err != nil || !access.Unrestricted {
			t.Fatalf("%s AccessFor ARM R: %v %+v", name, err, access)
		}
	}
}

func TestStore_FilePreferredOverENVString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	if err := os.WriteFile(path, []byte("fileuser|fp|IBIS=read\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New("envuser|ep|IBIS=write", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), "envuser", "ep"); err != domain.ErrUnauthorized {
		t.Fatalf("env user should be ignored when file set: %v", err)
	}
	u, err := store.Authenticate(context.Background(), "fileuser", "fp")
	if err != nil || !u.Can("IBIS", domain.AccessRead) {
		t.Fatalf("%v %+v", err, u)
	}
}

func TestStore_AccessForIgnoresARM(t *testing.T) {
	store, err := New("u|p|IBIS=read,write", "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, arm := range []domain.Workstation{domain.ARMCataloger, domain.ARMReader, domain.ARMAdministrator, ""} {
		access, err := store.AccessFor(context.Background(), "u", arm)
		uacl := domain.User{Databases: access.Databases}
		if err != nil || access.Unrestricted || !uacl.Can("IBIS", domain.AccessWrite) {
			t.Fatalf("arm=%q: %v %+v", arm, err, access)
		}
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

func TestStore_MissingFile(t *testing.T) {
	_, err := New("", filepath.Join(t.TempDir(), "nope.txt"), true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUsers_BadEntry(t *testing.T) {
	_, err := New("broken", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = New("u|p|IBIS=nope", "", true)
	if err == nil {
		t.Fatal("expected bad perm")
	}
}

func TestParseUsersFile_CRLF(t *testing.T) {
	users, err := parseUsersFile("a|b|IBIS=read\r\n# c\r\nd|e|*\r\nroot|root|\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 || !users["d"].Unrestricted || !users["root"].Unrestricted {
		t.Fatalf("%+v", users)
	}
}
