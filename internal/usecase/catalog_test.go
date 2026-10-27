package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/ivm97/irbis-api/internal/domain"
)

func TestBuildExpression_Modes(t *testing.T) {
	cases := []struct {
		name string
		q    domain.SearchQuery
		want string
	}{
		{"raw", domain.SearchQuery{Expression: `"A=TEST$"`}, `"A=TEST$"`},
		{"author", domain.SearchQuery{Mode: domain.ModeAuthor, Value: "Пушкин"}, "A=Пушкин$"},
		{"title trunc", domain.SearchQuery{Mode: domain.ModeTitle, Value: "Война$"}, "T=Война$"},
		{"all", domain.SearchQuery{Mode: domain.ModeAll}, "*"},
		{"empty mode value", domain.SearchQuery{Mode: domain.ModeKeyword}, "K=$"},
		{"default all", domain.SearchQuery{}, "*"},
		{"default expression", domain.SearchQuery{Expression: "*"}, "*"},
		{"wildcard kept", domain.SearchQuery{Mode: domain.ModeAuthor, Value: "Пуш*"}, "A=Пуш*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildExpression(tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBuildExpression_UnknownMode(t *testing.T) {
	_, err := BuildExpression(domain.SearchQuery{Mode: "nope"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestSearch_PaginationAndWorkstation(t *testing.T) {
	cat := &mockCatalog{
		searchRes: domain.SearchResult{Expression: "A=X$", Count: 1, Total: 50, Hits: []domain.SearchHit{{MFN: 10}}},
	}
	svc := testService(cat)

	res, err := svc.Search(context.Background(), "librarian", "secret", "R", "IBIS", domain.SearchQuery{
		Mode:        domain.ModeAuthor,
		Value:       "X",
		Limit:       5,
		Offset:      10,
		WithRecords: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 50 || res.Count != 1 {
		t.Fatalf("res=%+v", res)
	}
	if cat.lastQ.Expression != "A=X$" {
		t.Fatalf("expr=%q", cat.lastQ.Expression)
	}
	if cat.lastQ.Offset != 10 || cat.lastQ.Limit != 5 || !cat.lastQ.WithRecords {
		t.Fatalf("query=%+v", cat.lastQ)
	}
	if cat.lastSess.Workstation != domain.ARMReader {
		t.Fatalf("arm=%q", cat.lastSess.Workstation)
	}
}

func TestSearch_DefaultARMAndLimitClamp(t *testing.T) {
	cat := &mockCatalog{searchRes: domain.SearchResult{Hits: []domain.SearchHit{}}}
	svc := testService(cat)

	_, err := svc.Search(context.Background(), "librarian", "secret", "", "IBIS", domain.SearchQuery{
		Mode:  domain.ModeAll,
		Limit: 9999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cat.lastSess.Workstation != domain.ARMCataloger {
		t.Fatalf("arm=%q", cat.lastSess.Workstation)
	}
	if cat.lastQ.Limit != 500 {
		t.Fatalf("limit=%d", cat.lastQ.Limit)
	}
}

func TestSearch_ForbiddenAndUnauthorized(t *testing.T) {
	svc := testService(&mockCatalog{})
	_, err := svc.Search(context.Background(), "reader", "reader", "", "RDR", domain.SearchQuery{Mode: domain.ModeAll})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	_, err = svc.Search(context.Background(), "nope", "x", "", "IBIS", domain.SearchQuery{Mode: domain.ModeAll})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("want unauthorized, got %v", err)
	}
}

func TestMe(t *testing.T) {
	svc := testService(&mockCatalog{})
	me, err := svc.Me(context.Background(), "librarian", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if me.Username != "librarian" || len(me.Databases["IBIS"]) != 2 {
		t.Fatalf("%+v", me)
	}
}

func TestServerConfig(t *testing.T) {
	cat := &mockCatalog{info: domain.IrbisServerInfo{Version: "64", Databases: []domain.DatabaseInfo{{Name: "IBIS"}}}}
	svc := testService(cat)
	cfg, err := svc.ServerConfig(context.Background(), "librarian", "secret", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultWorkstation != "C" || cfg.Irbis.Version != "64" || len(cfg.SearchModes) == 0 {
		t.Fatalf("%+v", cfg)
	}
}

func TestServerConfig_IrbisDownStillOK(t *testing.T) {
	cat := &mockCatalog{infoErr: domain.ErrIrbis}
	svc := testService(cat)
	cfg, err := svc.ServerConfig(context.Background(), "librarian", "secret", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultWorkstation != "C" || len(cfg.Workstations) == 0 {
		t.Fatalf("%+v", cfg)
	}
}

func TestCRUD_Permissions(t *testing.T) {
	cat := &mockCatalog{
		record:  domain.Record{Fields: []domain.Field{{Tag: 200}}},
		saveRes: domain.Record{Fields: []domain.Field{{Tag: 200}}},
	}
	svc := testService(cat)

	_, err := svc.GetRecord(context.Background(), "reader", "reader", "", "IBIS", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveRecord(context.Background(), "reader", "reader", "", "IBIS", domain.RecordWrite{
		Fields: []domain.Field{{Tag: 200, Value: "x"}},
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("reader write: %v", err)
	}
	_, err = svc.SaveRecord(context.Background(), "librarian", "secret", "C", "IBIS", domain.RecordWrite{
		Fields: []domain.Field{{Tag: 200, Value: "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRecord(context.Background(), "librarian", "secret", "", "IBIS", 1); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalCorrection_BuildsExpressionAndValidates(t *testing.T) {
	cat := &mockCatalog{gblRes: domain.GblResult{Lines: []string{"ok"}}}
	svc := testService(cat)

	_, err := svc.GlobalCorrection(context.Background(), "librarian", "secret", "", "IBIS", domain.GblRequest{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty: %v", err)
	}

	_, err = svc.GlobalCorrection(context.Background(), "librarian", "secret", "", "IBIS", domain.GblRequest{
		Statements: []domain.GblStatement{{Command: "HACK"}},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("bad cmd: %v", err)
	}

	res, err := svc.GlobalCorrection(context.Background(), "librarian", "secret", "A", "IBIS", domain.GblRequest{
		Mode:      domain.ModeAuthor,
		Value:     "Test",
		MinMFN:    1,
		MaxMFN:    10,
		Actualize: true,
		Statements: []domain.GblStatement{
			{Command: "add", Parameter1: "907", Parameter2: "*", Format1: "'x'"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 {
		t.Fatalf("%+v", res)
	}
	if cat.lastGBL.SearchExpression != "A=Test$" {
		t.Fatalf("expr=%q", cat.lastGBL.SearchExpression)
	}
	if cat.lastGBL.Statements[0].Command != "ADD" {
		t.Fatalf("cmd=%q", cat.lastGBL.Statements[0].Command)
	}
	if cat.lastSess.Workstation != domain.ARMAdministrator {
		t.Fatalf("arm=%q", cat.lastSess.Workstation)
	}
}

func TestGlobalCorrection_ReadOnlyForbidden(t *testing.T) {
	svc := testService(&mockCatalog{})
	_, err := svc.GlobalCorrection(context.Background(), "reader", "reader", "", "IBIS", domain.GblRequest{
		Filename: "task.gbl",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("%v", err)
	}
}

func TestInvalidWorkstation(t *testing.T) {
	svc := testService(&mockCatalog{})
	_, err := svc.Search(context.Background(), "librarian", "secret", "Z", "IBIS", domain.SearchQuery{Mode: domain.ModeAll})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("%v", err)
	}
}

func TestBatchAndTerms(t *testing.T) {
	cat := &mockCatalog{terms: []domain.TermInfo{{Text: "A=X", Count: 2}}}
	svc := testService(cat)

	recs, err := svc.SaveRecords(context.Background(), "librarian", "secret", "", "IBIS", []domain.RecordWrite{
		{Fields: []domain.Field{{Tag: 200, Value: "a"}}},
		{Fields: []domain.Field{{Tag: 200, Value: "b"}}},
	})
	if err != nil || len(recs) != 2 {
		t.Fatalf("%v %+v", err, recs)
	}
	terms, err := svc.ListTerms(context.Background(), "librarian", "secret", "", "IBIS", "A=", 10)
	if err != nil || len(terms) != 1 {
		t.Fatalf("%v %+v", err, terms)
	}
}
