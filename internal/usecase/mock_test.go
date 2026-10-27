package usecase

import (
	"context"
	"sync"

	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

type memoryAuth struct {
	users map[string]domain.User
}

func (a memoryAuth) Authenticate(_ context.Context, username, password string) (domain.User, error) {
	u, ok := a.users[username]
	if !ok || u.Password != password {
		return domain.User{}, domain.ErrUnauthorized
	}
	out := u
	out.Password = ""
	return out, nil
}

type mockCatalog struct {
	mu sync.Mutex

	lastSess ports.Session
	lastDB   string
	lastQ    domain.SearchQuery
	lastGBL  domain.GblRequest

	searchRes domain.SearchResult
	searchErr error
	countN    int
	countErr  error
	record    domain.Record
	recordErr error
	saveRes   domain.Record
	saveErr   error
	batchRes  []domain.Record
	batchErr  error
	delErr    error
	terms     []domain.TermInfo
	termsErr  error
	gblRes    domain.GblResult
	gblErr    error
	info      domain.IrbisServerInfo
	infoErr   error
}

func (m *mockCatalog) Search(_ context.Context, sess ports.Session, database string, q domain.SearchQuery) (domain.SearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB, m.lastQ = sess, database, q
	return m.searchRes, m.searchErr
}

func (m *mockCatalog) Count(_ context.Context, sess ports.Session, database string, q domain.SearchQuery) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB, m.lastQ = sess, database, q
	return q.Expression, m.countN, m.countErr
}

func (m *mockCatalog) GetRecord(_ context.Context, sess ports.Session, database string, mfn int, _ string) (domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB = sess, database
	if m.recordErr != nil {
		return domain.Record{}, m.recordErr
	}
	r := m.record
	r.MFN = mfn
	r.Database = database
	return r, nil
}

func (m *mockCatalog) GetRecords(ctx context.Context, sess ports.Session, database string, mfns []int) ([]domain.Record, error) {
	out := make([]domain.Record, 0, len(mfns))
	for _, mfn := range mfns {
		r, err := m.GetRecord(ctx, sess, database, mfn, "")
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *mockCatalog) SaveRecord(_ context.Context, sess ports.Session, database string, in domain.RecordWrite) (domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB = sess, database
	if m.saveErr != nil {
		return domain.Record{}, m.saveErr
	}
	r := m.saveRes
	r.Database = database
	r.MFN = in.MFN
	if r.MFN == 0 {
		r.MFN = 99
	}
	r.Fields = in.Fields
	return r, nil
}

func (m *mockCatalog) SaveRecords(_ context.Context, sess ports.Session, database string, in []domain.RecordWrite) ([]domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB = sess, database
	if m.batchErr != nil {
		return nil, m.batchErr
	}
	if m.batchRes != nil {
		return m.batchRes, nil
	}
	out := make([]domain.Record, 0, len(in))
	for i, item := range in {
		out = append(out, domain.Record{Database: database, MFN: i + 1, Fields: item.Fields})
	}
	return out, nil
}

func (m *mockCatalog) DeleteRecord(_ context.Context, sess ports.Session, database string, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB = sess, database
	return m.delErr
}

func (m *mockCatalog) ListTerms(_ context.Context, sess ports.Session, database, _ string, _ int) ([]domain.TermInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB = sess, database
	return m.terms, m.termsErr
}

func (m *mockCatalog) GlobalCorrection(_ context.Context, sess ports.Session, database string, req domain.GblRequest) (domain.GblResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess, m.lastDB, m.lastGBL = sess, database, req
	return m.gblRes, m.gblErr
}

func (m *mockCatalog) ServerInfo(_ context.Context, sess ports.Session) (domain.IrbisServerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSess = sess
	return m.info, m.infoErr
}

func testService(cat *mockCatalog) *CatalogService {
	auth := memoryAuth{users: map[string]domain.User{
		"librarian": {
			Username: "librarian",
			Password: "secret",
			Databases: map[string][]domain.AccessLevel{
				"IBIS": {domain.AccessRead, domain.AccessWrite},
				"RDR":  {domain.AccessRead},
			},
		},
		"reader": {
			Username: "reader",
			Password: "reader",
			Databases: map[string][]domain.AccessLevel{
				"IBIS": {domain.AccessRead},
			},
		},
	}}
	return NewCatalogService(auth, cat, ports.Session{}, domain.ARMCataloger)
}
