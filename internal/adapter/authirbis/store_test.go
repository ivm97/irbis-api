package authirbis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ivm97/irbis-api/internal/adapter/irbisgw"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

type mockReader struct {
	users     []irbisgw.IrbisUserInfo
	usersErr  error
	usersN    atomic.Int32
	ini       map[string]string
	iniErr    error
	menus     map[string][]domain.DatabaseInfo
	menuErr   error
	menuCalls atomic.Int32
	menuMu    sync.Mutex
	menuLog   []string
}

func (m *mockReader) GetUserList(context.Context, ports.Session) ([]irbisgw.IrbisUserInfo, error) {
	m.usersN.Add(1)
	return m.users, m.usersErr
}

func (m *mockReader) ReadIniDBNameCat(_ context.Context, _ ports.Session, specification string) (string, error) {
	if m.iniErr != nil {
		return "", m.iniErr
	}
	return m.ini[specification], nil
}

func (m *mockReader) ReadDatabaseMenu(_ context.Context, _ ports.Session, specification string) ([]domain.DatabaseInfo, error) {
	m.menuCalls.Add(1)
	m.menuMu.Lock()
	m.menuLog = append(m.menuLog, specification)
	m.menuMu.Unlock()
	if m.menuErr != nil {
		return nil, m.menuErr
	}
	return m.menus[specification], nil
}

func testStore(t *testing.T, r *mockReader, opt ...Options) *Store {
	t.Helper()
	o := Options{
		Admin:         ports.Session{Username: "admin", Password: "admin", Workstation: domain.ARMAdministrator},
		DatabasesMenu: "1..dbnam2.mnu",
		IniPathCode:   "0",
		MenuPathCode:  "1",
		CacheTTL:      time.Minute,
	}
	if len(opt) > 0 {
		if opt[0].UserMenuPattern != "" {
			o.UserMenuPattern = opt[0].UserMenuPattern
		}
		if opt[0].CacheTTL > 0 {
			o.CacheTTL = opt[0].CacheTTL
		}
	}
	s, err := New(r, o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAuthenticateAndAccess_LazyMenu(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "ivanov",
			Password: "secret",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "IRBISC_IVANOV.INI"},
		}},
		ini: map[string]string{
			"0..IRBISC_IVANOV.INI": "db_ivanov.mnu",
		},
		menus: map[string][]domain.DatabaseInfo{
			"1..db_ivanov.mnu": {
				{Name: "RDR", ReadOnly: false},
				{Name: "KOBKN", ReadOnly: true},
			},
		},
	}
	s := testStore(t, r)

	u, err := s.Authenticate(context.Background(), "IVANOV", "secret")
	if err != nil || u.Username != "ivanov" {
		t.Fatalf("%v %+v", err, u)
	}
	if r.menuCalls.Load() != 0 {
		t.Fatal("menu should be lazy")
	}

	access, err := s.AccessFor(context.Background(), "ivanov", domain.ARMCataloger)
	if err != nil {
		t.Fatal(err)
	}
	uacl := domain.User{Databases: access.Databases}
	if !uacl.Can("RDR", domain.AccessWrite) {
		t.Fatalf("RDR write: %+v", access)
	}
	if !uacl.Can("KOBKN", domain.AccessRead) || uacl.Can("KOBKN", domain.AccessWrite) {
		t.Fatalf("KOBKN readonly: %+v", access)
	}
	if r.menuCalls.Load() != 1 {
		t.Fatalf("menu calls=%d", r.menuCalls.Load())
	}

	_, err = s.AccessFor(context.Background(), "ivanov", domain.ARMCataloger)
	if err != nil || r.menuCalls.Load() != 1 {
		t.Fatalf("cache miss? calls=%d err=%v", r.menuCalls.Load(), err)
	}
	if r.usersN.Load() != 1 {
		t.Fatalf("users reloads=%d", r.usersN.Load())
	}
}

func TestAccessFor_ARMDenied(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "ivanov",
			Password: "secret",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "IRBISC_IVANOV.INI"},
		}},
	}
	s := testStore(t, r)
	_, err := s.AccessFor(context.Background(), "ivanov", domain.ARMAdministrator)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("%v", err)
	}
}

func TestAccessFor_FallbackMenuWhenNoDBNNAMECAT(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "petrov",
			Password: "p",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "irbisc.ini"},
		}},
		ini: map[string]string{"0..irbisc.ini": ""},
		menus: map[string][]domain.DatabaseInfo{
			"1..dbnam2.mnu": {{Name: "IBIS", ReadOnly: false}},
		},
	}
	s := testStore(t, r)
	access, err := s.AccessFor(context.Background(), "petrov", domain.ARMCataloger)
	if err != nil {
		t.Fatal(err)
	}
	uacl := domain.User{Databases: access.Databases}
	if !uacl.Can("IBIS", domain.AccessWrite) {
		t.Fatalf("%+v", access)
	}
}

func TestAccessFor_EmptyPersonalFallsBackToGlobal(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "u",
			Password: "p",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "IRBISC_U.INI"},
		}},
		ini: map[string]string{"0..IRBISC_U.INI": "db_u.mnu"},
		menus: map[string][]domain.DatabaseInfo{
			"1..db_u.mnu":    nil,
			"1..dbnam2.mnu": {{Name: "IBIS", ReadOnly: false}},
		},
	}
	s := testStore(t, r)
	access, err := s.AccessFor(context.Background(), "u", domain.ARMCataloger)
	if err != nil {
		t.Fatal(err)
	}
	if r.menuCalls.Load() != 2 {
		t.Fatalf("want personal+fallback, got %d %v", r.menuCalls.Load(), r.menuLog)
	}
	uacl := domain.User{Databases: access.Databases}
	if !uacl.Can("IBIS", domain.AccessWrite) {
		t.Fatalf("%+v", access)
	}
}

func TestAccessFor_UserMenuPattern(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "Sid",
			Password: "p",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "IRBISC_SID.INI"},
		}},
		ini: map[string]string{"0..IRBISC_SID.INI": ""},
		menus: map[string][]domain.DatabaseInfo{
			"1..db_Sid.mnu": {{Name: "RDR", ReadOnly: true}},
		},
	}
	s := testStore(t, r, Options{UserMenuPattern: "db_{user}.mnu"})
	access, err := s.AccessFor(context.Background(), "Sid", domain.ARMCataloger)
	if err != nil {
		t.Fatal(err)
	}
	uacl := domain.User{Databases: access.Databases}
	if !uacl.Can("RDR", domain.AccessRead) {
		t.Fatalf("%+v log=%v", access, r.menuLog)
	}
}

func TestAccessFor_FullSpecInDBNNAMECAT(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "u",
			Password: "p",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "x.ini"},
		}},
		ini: map[string]string{"0..x.ini": "1..custom.mnu"},
		menus: map[string][]domain.DatabaseInfo{
			"1..custom.mnu": {{Name: "ZZZ", ReadOnly: false}},
		},
	}
	s := testStore(t, r)
	access, err := s.AccessFor(context.Background(), "u", domain.ARMCataloger)
	uacl := domain.User{Databases: access.Databases}
	if err != nil || !uacl.Can("ZZZ", domain.AccessWrite) {
		t.Fatalf("%v %+v", err, access)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{Name: "u", Password: "right", ArmIni: map[domain.Workstation]string{domain.ARMCataloger: "x.ini"}}},
	}
	s := testStore(t, r)
	_, err := s.Authenticate(context.Background(), "u", "wrong")
	if err != domain.ErrUnauthorized {
		t.Fatalf("%v", err)
	}
}

func TestNew_Validation(t *testing.T) {
	if _, err := New(nil, Options{Admin: ports.Session{Username: "a", Password: "b"}}); err == nil {
		t.Fatal("reader")
	}
	if _, err := New(&mockReader{}, Options{}); err == nil {
		t.Fatal("admin")
	}
}

func TestACLCacheTTLExpiry(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{
			Name:     "u",
			Password: "p",
			ArmIni:   map[domain.Workstation]string{domain.ARMCataloger: "x.ini"},
		}},
		ini:   map[string]string{"0..x.ini": "db.mnu"},
		menus: map[string][]domain.DatabaseInfo{"1..db.mnu": {{Name: "IBIS"}}},
	}
	s := testStore(t, r, Options{CacheTTL: 20 * time.Millisecond})
	if _, err := s.AccessFor(context.Background(), "u", domain.ARMCataloger); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := s.AccessFor(context.Background(), "u", domain.ARMCataloger); err != nil {
		t.Fatal(err)
	}
	if r.menuCalls.Load() < 2 {
		t.Fatalf("expected reload after TTL, calls=%d", r.menuCalls.Load())
	}
}

func TestSingleflightUsers(t *testing.T) {
	r := &mockReader{
		users: []irbisgw.IrbisUserInfo{{Name: "u", Password: "p", ArmIni: map[domain.Workstation]string{domain.ARMCataloger: "x.ini"}}},
	}
	s := testStore(t, r)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Authenticate(context.Background(), "u", "p")
		}()
	}
	wg.Wait()
	if r.usersN.Load() != 1 {
		t.Fatalf("singleflight users: %d", r.usersN.Load())
	}
}

func TestHelpers(t *testing.T) {
	if expandUserPattern("db_{USER}.mnu", "ivan") != "db_IVAN.mnu" {
		t.Fatal(expandUserPattern("db_{USER}.mnu", "ivan"))
	}
	if fileSpec("1", "a.mnu") != "1..a.mnu" {
		t.Fatal(fileSpec("1", "a.mnu"))
	}
	s := &Store{databasesMenu: "1..dbnam2.mnu", menuPathCode: "1"}
	if s.menuFileSpec("1..x.mnu") != "1..x.mnu" {
		t.Fatal(s.menuFileSpec("1..x.mnu"))
	}
	if s.menuFileSpec("x.mnu") != "1..x.mnu" {
		t.Fatal(s.menuFileSpec("x.mnu"))
	}
}
