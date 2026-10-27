package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
	"github.com/ivm97/irbis-api/internal/printtpl"
)

// CatalogService orchestrates authz + catalog operations.
type CatalogService struct {
	auth               ports.AuthStore
	catalog            ports.Catalog
	irbis              ports.Session // service account when UseRequestCredentials=false; Username empty = use API user
	defaultWorkstation domain.Workstation
	templates          *printtpl.Store
}

func NewCatalogService(auth ports.AuthStore, catalog ports.Catalog, irbis ports.Session, defaultWorkstation domain.Workstation) *CatalogService {
	return NewCatalogServiceWithTemplates(auth, catalog, irbis, defaultWorkstation, "templates")
}

func NewCatalogServiceWithTemplates(auth ports.AuthStore, catalog ports.Catalog, irbis ports.Session, defaultWorkstation domain.Workstation, templatesDir string) *CatalogService {
	if defaultWorkstation == "" {
		defaultWorkstation = domain.ARMCataloger
	}
	return &CatalogService{
		auth:               auth,
		catalog:            catalog,
		irbis:              irbis,
		defaultWorkstation: defaultWorkstation,
		templates:          printtpl.NewStore(templatesDir),
	}
}

func (s *CatalogService) DefaultWorkstation() domain.Workstation {
	return s.defaultWorkstation
}

func (s *CatalogService) authenticate(ctx context.Context, username, password string, workstation string) (domain.User, ports.Session, error) {
	user, err := s.auth.Authenticate(ctx, username, password)
	if err != nil {
		return domain.User{}, ports.Session{}, err
	}
	arm, err := domain.NormalizeWorkstation(workstation, s.defaultWorkstation)
	if err != nil {
		return domain.User{}, ports.Session{}, err
	}
	sess := ports.Session{
		Username:    username,
		Password:    password,
		Workstation: arm,
	}
	if s.irbis.Username != "" {
		sess.Username = s.irbis.Username
		sess.Password = s.irbis.Password
	}
	return user, sess, nil
}

func (s *CatalogService) require(user domain.User, database string, level domain.AccessLevel) error {
	if !user.Can(database, level) {
		return fmt.Errorf("%w: no %s access to database %q", domain.ErrForbidden, level, database)
	}
	return nil
}

func BuildExpression(q domain.SearchQuery) (string, error) {
	mode := q.Mode
	if mode == "" {
		if q.Expression != "" {
			mode = domain.ModeExpression
		} else {
			mode = domain.ModeAll
		}
	}

	switch mode {
	case domain.ModeExpression:
		if strings.TrimSpace(q.Expression) == "" {
			return "", fmt.Errorf("%w: expression is required", domain.ErrInvalidInput)
		}
		return q.Expression, nil
	case domain.ModeAll:
		return `*`, nil
	case domain.ModeAuthor:
		return wrapPrefix("A=", q.Value), nil
	case domain.ModeTitle:
		return wrapPrefix("T=", q.Value), nil
	case domain.ModeKeyword:
		return wrapPrefix("K=", q.Value), nil
	case domain.ModeInventory:
		return wrapPrefix("IN=", q.Value), nil
	case domain.ModeIndex:
		return wrapPrefix("I=", q.Value), nil
	case domain.ModeYear:
		return wrapPrefix("G=", q.Value), nil
	case domain.ModePublisher:
		return wrapPrefix("O=", q.Value), nil
	case domain.ModeSubject:
		return wrapPrefix("S=", q.Value), nil
	case domain.ModeLanguage:
		return wrapPrefix("J=", q.Value), nil
	default:
		return "", fmt.Errorf("%w: unknown search mode %q", domain.ErrInvalidInput, mode)
	}
}

func wrapPrefix(prefix, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return prefix + "$"
	}
	if strings.HasSuffix(v, "$") || strings.ContainsAny(v, "*?") {
		return prefix + v
	}
	return prefix + v + "$"
}

func (s *CatalogService) Me(ctx context.Context, apiUser, apiPass string) (domain.MeResponse, error) {
	user, err := s.auth.Authenticate(ctx, apiUser, apiPass)
	if err != nil {
		return domain.MeResponse{}, err
	}
	dbs := make(map[string][]domain.AccessLevel, len(user.Databases))
	for name, levels := range user.Databases {
		cp := make([]domain.AccessLevel, len(levels))
		copy(cp, levels)
		dbs[name] = cp
	}
	return domain.MeResponse{Username: user.Username, Databases: dbs}, nil
}

func (s *CatalogService) ServerConfig(ctx context.Context, apiUser, apiPass, workstation string) (domain.ServerConfigResponse, error) {
	_, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return domain.ServerConfigResponse{}, err
	}
	info, err := s.catalog.ServerInfo(ctx, sess)
	if err != nil {
		// Still return static config if IRBIS is unreachable.
		return domain.ServerConfigResponse{
			DefaultWorkstation: string(s.defaultWorkstation),
			Workstations:       domain.KnownWorkstations,
			SearchModes:        domain.AllSearchModes(),
			Irbis:              domain.IrbisServerInfo{},
		}, nil
	}
	return domain.ServerConfigResponse{
		DefaultWorkstation: string(s.defaultWorkstation),
		Workstations:       domain.KnownWorkstations,
		SearchModes:        domain.AllSearchModes(),
		Irbis:              info,
	}, nil
}

func (s *CatalogService) Search(ctx context.Context, apiUser, apiPass, workstation, database string, q domain.SearchQuery) (domain.SearchResult, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return domain.SearchResult{}, err
	}
	if err := s.require(user, database, domain.AccessRead); err != nil {
		return domain.SearchResult{}, err
	}
	expr, err := BuildExpression(q)
	if err != nil {
		return domain.SearchResult{}, err
	}
	q.Expression = expr
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 500 {
		q.Limit = 500
	}
	return s.catalog.Search(ctx, sess, database, q)
}

func (s *CatalogService) Count(ctx context.Context, apiUser, apiPass, workstation, database string, q domain.SearchQuery) (string, int, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return "", 0, err
	}
	if err := s.require(user, database, domain.AccessRead); err != nil {
		return "", 0, err
	}
	expr, err := BuildExpression(q)
	if err != nil {
		return "", 0, err
	}
	q.Expression = expr
	return s.catalog.Count(ctx, sess, database, q)
}

func (s *CatalogService) GetRecord(ctx context.Context, apiUser, apiPass, workstation, database string, mfn int, format string) (domain.Record, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return domain.Record{}, err
	}
	if err := s.require(user, database, domain.AccessRead); err != nil {
		return domain.Record{}, err
	}
	if mfn <= 0 {
		return domain.Record{}, fmt.Errorf("%w: mfn must be positive", domain.ErrInvalidInput)
	}
	return s.catalog.GetRecord(ctx, sess, database, mfn, format)
}

func (s *CatalogService) SaveRecord(ctx context.Context, apiUser, apiPass, workstation, database string, in domain.RecordWrite) (domain.Record, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return domain.Record{}, err
	}
	if err := s.require(user, database, domain.AccessWrite); err != nil {
		return domain.Record{}, err
	}
	if len(in.Fields) == 0 {
		return domain.Record{}, fmt.Errorf("%w: fields required", domain.ErrInvalidInput)
	}
	return s.catalog.SaveRecord(ctx, sess, database, in)
}

func (s *CatalogService) SaveRecords(ctx context.Context, apiUser, apiPass, workstation, database string, in []domain.RecordWrite) ([]domain.Record, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return nil, err
	}
	if err := s.require(user, database, domain.AccessWrite); err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, fmt.Errorf("%w: records required", domain.ErrInvalidInput)
	}
	if len(in) > 100 {
		return nil, fmt.Errorf("%w: max 100 records per batch", domain.ErrInvalidInput)
	}
	for i, r := range in {
		if len(r.Fields) == 0 {
			return nil, fmt.Errorf("%w: records[%d].fields required", domain.ErrInvalidInput, i)
		}
	}
	return s.catalog.SaveRecords(ctx, sess, database, in)
}

func (s *CatalogService) DeleteRecord(ctx context.Context, apiUser, apiPass, workstation, database string, mfn int) error {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return err
	}
	if err := s.require(user, database, domain.AccessWrite); err != nil {
		return err
	}
	if mfn <= 0 {
		return fmt.Errorf("%w: mfn must be positive", domain.ErrInvalidInput)
	}
	return s.catalog.DeleteRecord(ctx, sess, database, mfn)
}

func (s *CatalogService) ListTerms(ctx context.Context, apiUser, apiPass, workstation, database, start string, limit int) ([]domain.TermInfo, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return nil, err
	}
	if err := s.require(user, database, domain.AccessRead); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.catalog.ListTerms(ctx, sess, database, start, limit)
}

var allowedGblCommands = map[string]struct{}{
	"ADD": {}, "DEL": {}, "REP": {}, "CHA": {}, "CHAC": {},
	"DELR": {}, "UNDELR": {}, "CORREC": {}, "NEWMFN": {}, "EMPTY": {},
	"UNDOR": {}, "END": {}, "IF": {}, "FI": {}, "ALL": {},
	"REPEAT": {}, "UNTIL": {}, "PUTLOG": {},
}

func (s *CatalogService) GlobalCorrection(ctx context.Context, apiUser, apiPass, workstation, database string, req domain.GblRequest) (domain.GblResult, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return domain.GblResult{}, err
	}
	if err := s.require(user, database, domain.AccessWrite); err != nil {
		return domain.GblResult{}, err
	}

	if req.Filename == "" && len(req.Statements) == 0 {
		return domain.GblResult{}, fmt.Errorf("%w: filename or statements required", domain.ErrInvalidInput)
	}
	for i, st := range req.Statements {
		cmd := strings.ToUpper(strings.TrimSpace(st.Command))
		if cmd == "" {
			return domain.GblResult{}, fmt.Errorf("%w: statements[%d].command required", domain.ErrInvalidInput, i)
		}
		if _, ok := allowedGblCommands[cmd]; !ok {
			return domain.GblResult{}, fmt.Errorf("%w: unsupported GBL command %q", domain.ErrInvalidInput, st.Command)
		}
		req.Statements[i].Command = cmd
	}

	// Build search expression from mode/value when raw search_expression empty.
	if strings.TrimSpace(req.SearchExpression) == "" {
		expr, err := BuildExpression(domain.SearchQuery{
			Mode:  req.Mode,
			Value: req.Value,
		})
		if err != nil {
			return domain.GblResult{}, err
		}
		req.SearchExpression = expr
	}

	if req.FirstRecord < 0 {
		req.FirstRecord = 0
	}
	if req.NumberOfRecords < 0 {
		req.NumberOfRecords = 0
	}
	if len(req.MfnList) > 10000 {
		return domain.GblResult{}, fmt.Errorf("%w: mfn_list too large", domain.ErrInvalidInput)
	}

	return s.catalog.GlobalCorrection(ctx, sess, database, req)
}

// PrintRequest selects records and a template for RTF fill.
type PrintRequest struct {
	Template string
	MFNs     []int
	Extras   map[string]string
}

// PrintResult is a filled document ready for download.
type PrintResult struct {
	Filename    string
	ContentType string
	Body        []byte
}

func (s *CatalogService) ListPrintTemplates() ([]string, error) {
	if s.templates == nil {
		return nil, fmt.Errorf("%w: templates not configured", domain.ErrInvalidInput)
	}
	return s.templates.List()
}

func (s *CatalogService) Print(ctx context.Context, apiUser, apiPass, workstation, database string, req PrintRequest) (PrintResult, error) {
	user, sess, err := s.authenticate(ctx, apiUser, apiPass, workstation)
	if err != nil {
		return PrintResult{}, err
	}
	if err := s.require(user, database, domain.AccessRead); err != nil {
		return PrintResult{}, err
	}
	tpl := strings.TrimSpace(req.Template)
	if tpl == "" {
		return PrintResult{}, fmt.Errorf("%w: template required", domain.ErrInvalidInput)
	}
	if len(req.MFNs) == 0 {
		return PrintResult{}, fmt.Errorf("%w: mfns required", domain.ErrInvalidInput)
	}
	if len(req.MFNs) > 500 {
		return PrintResult{}, fmt.Errorf("%w: max 500 mfns per print", domain.ErrInvalidInput)
	}
	for _, m := range req.MFNs {
		if m <= 0 {
			return PrintResult{}, fmt.Errorf("%w: mfn must be positive", domain.ErrInvalidInput)
		}
	}

	recs, err := s.catalog.GetRecords(ctx, sess, database, req.MFNs)
	if err != nil {
		return PrintResult{}, err
	}
	if len(recs) == 0 {
		return PrintResult{}, domain.ErrNotFound
	}

	extras := printtpl.DocExtras(req.Extras)
	var rows []map[string]string
	if s.templates.HasRepeating(tpl) {
		for _, rec := range recs {
			rows = append(rows, printtpl.InventoryRows(rec)...)
		}
	} else {
		for _, rec := range recs {
			rows = append(rows, printtpl.CardKeys(rec))
		}
	}

	body, ext, err := s.templates.Render(tpl, rows, extras)
	if err != nil {
		return PrintResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	filename := printtpl.DefaultFilename(strings.TrimSuffix(filepath.Base(tpl), filepath.Ext(tpl)), len(recs)) + ext
	ct := "application/rtf"
	if strings.EqualFold(ext, ".txt") {
		ct = "text/plain; charset=windows-1251"
	}
	return PrintResult{Filename: filename, ContentType: ct, Body: body}, nil
}
