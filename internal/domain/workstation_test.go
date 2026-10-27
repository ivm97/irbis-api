package domain

import (
	"errors"
	"testing"
)

func TestNormalizeWorkstation(t *testing.T) {
	got, err := NormalizeWorkstation("", ARMCataloger)
	if err != nil || got != ARMCataloger {
		t.Fatalf("empty: %v %q", err, got)
	}
	got, err = NormalizeWorkstation("r", ARMCataloger)
	if err != nil || got != ARMReader {
		t.Fatalf("r: %v %q", err, got)
	}
	_, err = NormalizeWorkstation("Z", ARMCataloger)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Z: %v", err)
	}
	_, err = NormalizeWorkstation("CC", ARMCataloger)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CC: %v", err)
	}
}

func TestUserCan(t *testing.T) {
	u := User{Databases: map[string][]AccessLevel{
		"IBIS": {AccessWrite},
	}}
	if !u.Can("ibis", AccessRead) {
		t.Fatal("write should imply read")
	}
	if !u.Can("IBIS", AccessWrite) {
		t.Fatal("write")
	}
	if u.Can("RDR", AccessRead) {
		t.Fatal("missing db")
	}
}
