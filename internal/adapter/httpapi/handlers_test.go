package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
	"github.com/ivm97/irbis-api/internal/usecase"
)

type memAuth struct{}

func (memAuth) Authenticate(_ context.Context, username, password string) (domain.User, error) {
	if username == "librarian" && password == "secret" {
		return domain.User{
			Username: "librarian",
			Databases: map[string][]domain.AccessLevel{
				"IBIS": {domain.AccessRead, domain.AccessWrite},
			},
		}, nil
	}
	if username == "reader" && password == "reader" {
		return domain.User{
			Username:  "reader",
			Databases: map[string][]domain.AccessLevel{"IBIS": {domain.AccessRead}},
		}, nil
	}
	return domain.User{}, domain.ErrUnauthorized
}

type fakeCatalog struct {
	search domain.SearchResult
	info   domain.IrbisServerInfo
	gbl    domain.GblResult
	lastWS domain.Workstation
	lastQ  domain.SearchQuery
}

func (f *fakeCatalog) Search(_ context.Context, sess ports.Session, _ string, q domain.SearchQuery) (domain.SearchResult, error) {
	f.lastWS, f.lastQ = sess.Workstation, q
	res := f.search
	res.Expression = q.Expression
	return res, nil
}
func (f *fakeCatalog) Count(_ context.Context, sess ports.Session, _ string, q domain.SearchQuery) (string, int, error) {
	f.lastWS = sess.Workstation
	return q.Expression, 7, nil
}
func (f *fakeCatalog) GetRecord(_ context.Context, sess ports.Session, database string, mfn int, _ string) (domain.Record, error) {
	f.lastWS = sess.Workstation
	return domain.Record{Database: database, MFN: mfn, Fields: []domain.Field{{Tag: 200, Subfields: []domain.SubField{{Code: "a", Value: "T"}}}}}, nil
}
func (f *fakeCatalog) SaveRecord(_ context.Context, sess ports.Session, database string, in domain.RecordWrite) (domain.Record, error) {
	f.lastWS = sess.Workstation
	mfn := in.MFN
	if mfn == 0 {
		mfn = 42
	}
	return domain.Record{Database: database, MFN: mfn, Fields: in.Fields}, nil
}
func (f *fakeCatalog) SaveRecords(_ context.Context, sess ports.Session, database string, in []domain.RecordWrite) ([]domain.Record, error) {
	f.lastWS = sess.Workstation
	out := make([]domain.Record, len(in))
	for i := range in {
		out[i] = domain.Record{Database: database, MFN: i + 1, Fields: in[i].Fields}
	}
	return out, nil
}
func (f *fakeCatalog) DeleteRecord(_ context.Context, sess ports.Session, _ string, _ int) error {
	f.lastWS = sess.Workstation
	return nil
}
func (f *fakeCatalog) ListTerms(_ context.Context, sess ports.Session, _, _ string, _ int) ([]domain.TermInfo, error) {
	f.lastWS = sess.Workstation
	return []domain.TermInfo{{Text: "A=X", Count: 1}}, nil
}
func (f *fakeCatalog) GlobalCorrection(_ context.Context, sess ports.Session, _ string, _ domain.GblRequest) (domain.GblResult, error) {
	f.lastWS = sess.Workstation
	return f.gbl, nil
}
func (f *fakeCatalog) ServerInfo(_ context.Context, sess ports.Session) (domain.IrbisServerInfo, error) {
	f.lastWS = sess.Workstation
	return f.info, nil
}

func newTestEcho(t *testing.T) (*echo.Echo, *fakeCatalog) {
	t.Helper()
	cat := &fakeCatalog{
		search: domain.SearchResult{Count: 1, Total: 100, Hits: []domain.SearchHit{{MFN: 5}}},
		info:   domain.IrbisServerInfo{Version: "2014.1"},
		gbl:    domain.GblResult{Lines: []string{"done"}},
	}
	svc := usecase.NewCatalogService(memAuth{}, cat, ports.Session{}, domain.ARMCataloger)
	e := echo.New()
	NewHandler(svc).Register(e)
	return e, cat
}

func doReq(t *testing.T, e *echo.Echo, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func auth() map[string]string {
	return map[string]string{"X-API-User": "librarian", "X-API-Password": "secret"}
}

func TestHealthAndSwagger(t *testing.T) {
	e, _ := newTestEcho(t)
	rec := doReq(t, e, http.MethodGet, "/health", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, e, http.MethodGet, "/swagger", nil, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("swagger-ui")) {
		t.Fatalf("%d", rec.Code)
	}
}

func TestMeAndConfig(t *testing.T) {
	e, cat := newTestEcho(t)
	rec := doReq(t, e, http.MethodGet, "/v1/me", nil, auth())
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var me domain.MeResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Username != "librarian" {
		t.Fatalf("%+v", me)
	}

	h := auth()
	h["X-Irbis-Workstation"] = "R"
	rec = doReq(t, e, http.MethodGet, "/v1/config", nil, h)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if cat.lastWS != domain.ARMReader {
		t.Fatalf("ws=%q", cat.lastWS)
	}
}

func TestSearchPaginationHeaders(t *testing.T) {
	e, cat := newTestEcho(t)
	h := auth()
	h["X-Irbis-Workstation"] = "R"
	rec := doReq(t, e, http.MethodGet, "/v1/databases/IBIS/records/search?mode=author&value=Pushkin&limit=5&offset=20&with_records=true", nil, h)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var res domain.SearchResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Total != 100 || cat.lastQ.Offset != 20 || cat.lastQ.Limit != 5 || !cat.lastQ.WithRecords {
		t.Fatalf("res=%+v q=%+v", res, cat.lastQ)
	}
	if cat.lastWS != domain.ARMReader {
		t.Fatalf("ws=%q", cat.lastWS)
	}
}

func TestRecordCRUDAndGBL(t *testing.T) {
	e, _ := newTestEcho(t)
	h := auth()

	rec := doReq(t, e, http.MethodGet, "/v1/databases/IBIS/records/1", nil, h)
	if rec.Code != 200 {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, e, http.MethodPost, "/v1/databases/IBIS/records", domain.RecordWrite{
		Fields: []domain.Field{{Tag: 200, Subfields: []domain.SubField{{Code: "a", Value: "New"}}}},
	}, h)
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, e, http.MethodPut, "/v1/databases/IBIS/records/5", domain.RecordWrite{
		Fields: []domain.Field{{Tag: 200, Value: "Upd"}},
	}, h)
	if rec.Code != 200 {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, e, http.MethodDelete, "/v1/databases/IBIS/records/5", nil, h)
	if rec.Code != 204 {
		t.Fatalf("del %d", rec.Code)
	}

	rec = doReq(t, e, http.MethodPost, "/v1/databases/IBIS/records/gbl", domain.GblRequest{
		Mode:  domain.ModeAll,
		Statements: []domain.GblStatement{
			{Command: "ADD", Parameter1: "907", Parameter2: "*", Format1: "'x'"},
		},
	}, h)
	if rec.Code != 200 {
		t.Fatalf("gbl %d %s", rec.Code, rec.Body.String())
	}
}

func TestForbiddenWrite(t *testing.T) {
	e, _ := newTestEcho(t)
	h := map[string]string{"X-API-User": "reader", "X-API-Password": "reader"}
	rec := doReq(t, e, http.MethodPost, "/v1/databases/IBIS/records", domain.RecordWrite{
		Fields: []domain.Field{{Tag: 200, Value: "x"}},
	}, h)
	if rec.Code != 403 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
