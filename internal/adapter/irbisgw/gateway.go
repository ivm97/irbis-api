package irbisgw

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/ivm97/GoIrbis/irbis"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
)

// Gateway talks to IRBIS64 via GoIrbis Client (connect-per-call).
type Gateway struct {
	host          string
	port          int
	databasesMenu string
	dialTimeout   time.Duration
	ioTimeout     time.Duration
}

type Options struct {
	Host          string
	Port          int
	DatabasesMenu string
	DialTimeout   time.Duration
	IOTimeout     time.Duration
}

func New(opt Options) *Gateway {
	if opt.Port == 0 {
		opt.Port = 6666
	}
	if opt.DialTimeout == 0 {
		opt.DialTimeout = 10 * time.Second
	}
	if opt.DatabasesMenu == "" {
		opt.DatabasesMenu = "1..dbnam2.mnu"
	}
	return &Gateway{
		host:          opt.Host,
		port:          opt.Port,
		databasesMenu: opt.DatabasesMenu,
		dialTimeout:   opt.DialTimeout,
		ioTimeout:     opt.IOTimeout,
	}
}

func (g *Gateway) client(sess ports.Session, database string) *irbis.Client {
	ws := string(sess.Workstation)
	if ws == "" {
		ws = string(domain.ARMCataloger)
	}
	return irbis.NewClient(irbis.Config{
		Host:        g.host,
		Port:        g.port,
		Username:    sess.Username,
		Password:    sess.Password,
		Database:    database,
		Workstation: ws,
		DialTimeout: g.dialTimeout,
		IOTimeout:   g.ioTimeout,
	})
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", domain.ErrIrbis, err)
}

func toDomain(database string, r *irbis.MarcRecord) domain.Record {
	out := domain.Record{
		Database: database,
		MFN:      r.Mfn,
		Status:   r.Status,
		Version:  r.Version,
		Fields:   make([]domain.Field, 0, len(r.Fields)),
	}
	for _, f := range r.Fields {
		if f == nil {
			continue
		}
		df := domain.Field{Tag: f.Tag, Value: f.Value}
		if len(f.Subfields) > 0 {
			df.Subfields = make([]domain.SubField, 0, len(f.Subfields))
			for _, sf := range f.Subfields {
				if sf == nil {
					continue
				}
				df.Subfields = append(df.Subfields, domain.SubField{
					Code:  string(sf.Code),
					Value: sf.Value,
				})
			}
			df.Value = ""
		}
		out.Fields = append(out.Fields, df)
	}
	return out
}

func subfieldCode(code string) rune {
	if code == "" {
		return 'a'
	}
	r, _ := utf8.DecodeRuneInString(code)
	if r == utf8.RuneError {
		return 'a'
	}
	return r
}

func fromWrite(database string, in domain.RecordWrite) *irbis.MarcRecord {
	r := irbis.NewMarcRecord()
	r.Database = database
	r.Mfn = in.MFN
	r.Status = in.Status
	r.Version = in.Version
	for _, f := range in.Fields {
		if len(f.Subfields) == 0 {
			r.Add(f.Tag, f.Value)
			continue
		}
		field := irbis.NewRecordField(f.Tag, f.Value)
		for _, sf := range f.Subfields {
			field.Add(subfieldCode(sf.Code), sf.Value)
		}
		r.Fields = append(r.Fields, field)
	}
	return r
}

func (g *Gateway) Search(ctx context.Context, sess ports.Session, database string, q domain.SearchQuery) (domain.SearchResult, error) {
	c := g.client(sess, database)

	first := q.Offset + 1
	if first < 1 {
		first = 1
	}

	if q.WithRecords && q.Format == "" {
		out, err := c.SearchReadFull(ctx, q.Expression, q.Limit, q.Offset)
		if err != nil {
			return domain.SearchResult{}, mapErr(err)
		}
		hits := make([]domain.SearchHit, 0, len(out.Records))
		for i := range out.Records {
			dr := toDomain(database, &out.Records[i])
			hits = append(hits, domain.SearchHit{MFN: dr.MFN, Record: &dr})
		}
		return domain.SearchResult{
			Expression: q.Expression,
			Count:      len(hits),
			Total:      out.Total,
			Hits:       hits,
		}, nil
	}

	params := &irbis.SearchParameters{
		Database:        database,
		Expression:      q.Expression,
		NumberOfRecords: q.Limit,
		FirstRecord:     first,
		Format:          q.Format,
		MinMfn:          q.MinMFN,
		MaxMfn:          q.MaxMFN,
		Sequential:      q.Sequential,
	}

	outcome, err := c.SearchFull(ctx, params)
	if err != nil {
		return domain.SearchResult{}, mapErr(err)
	}

	hits := make([]domain.SearchHit, 0, len(outcome.Found))
	for _, item := range outcome.Found {
		hits = append(hits, domain.SearchHit{
			MFN:         item.Mfn,
			Description: item.Description,
		})
	}
	return domain.SearchResult{
		Expression: q.Expression,
		Count:      len(hits),
		Total:      outcome.Total,
		Hits:       hits,
	}, nil
}

func (g *Gateway) Count(ctx context.Context, sess ports.Session, database string, q domain.SearchQuery) (string, int, error) {
	c := g.client(sess, database)
	n, err := c.SearchCount(ctx, q.Expression)
	if err != nil {
		return q.Expression, 0, mapErr(err)
	}
	return q.Expression, n, nil
}

func (g *Gateway) GetRecord(ctx context.Context, sess ports.Session, database string, mfn int, format string) (domain.Record, error) {
	c := g.client(sess, database)
	var out domain.Record
	err := c.Do(ctx, func(conn *irbis.Connection) error {
		rec := conn.ReadRecord(mfn)
		if rec == nil {
			if e := conn.Err(); e != nil {
				return e
			}
			return domain.ErrNotFound
		}
		if rec.Mfn == 0 {
			return domain.ErrNotFound
		}
		out = toDomain(database, rec)
		if format != "" {
			out.Formatted = conn.FormatMfn(format, mfn)
			return conn.Err()
		}
		return nil
	})
	if err != nil {
		if err == domain.ErrNotFound {
			return domain.Record{}, domain.ErrNotFound
		}
		return domain.Record{}, mapErr(err)
	}
	return out, nil
}

func (g *Gateway) GetRecords(ctx context.Context, sess ports.Session, database string, mfns []int) ([]domain.Record, error) {
	if len(mfns) == 0 {
		return nil, nil
	}
	c := g.client(sess, database)
	raw, err := c.ReadRecords(ctx, mfns)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Record, 0, len(raw))
	for i := range raw {
		if raw[i].Mfn == 0 {
			continue
		}
		out = append(out, toDomain(database, &raw[i]))
	}
	return out, nil
}

func (g *Gateway) SaveRecord(ctx context.Context, sess ports.Session, database string, in domain.RecordWrite) (domain.Record, error) {
	c := g.client(sess, database)
	rec := fromWrite(database, in)
	_, err := c.WriteRecord(ctx, rec)
	if err != nil {
		return domain.Record{}, mapErr(err)
	}
	return toDomain(database, rec), nil
}

func (g *Gateway) SaveRecords(ctx context.Context, sess ports.Session, database string, in []domain.RecordWrite) ([]domain.Record, error) {
	c := g.client(sess, database)
	records := make([]irbis.MarcRecord, 0, len(in))
	for _, item := range in {
		records = append(records, *fromWrite(database, item))
	}
	err := c.Do(ctx, func(conn *irbis.Connection) error {
		if !conn.WriteRecords(records) {
			if e := conn.Err(); e != nil {
				return e
			}
			return fmt.Errorf("write records failed")
		}
		return nil
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Record, 0, len(records))
	for i := range records {
		out = append(out, toDomain(database, &records[i]))
	}
	return out, nil
}

func (g *Gateway) DeleteRecord(ctx context.Context, sess ports.Session, database string, mfn int) error {
	c := g.client(sess, database)
	return mapErr(c.Do(ctx, func(conn *irbis.Connection) error {
		conn.DeleteRecord(mfn)
		return conn.Err()
	}))
}

func (g *Gateway) ListTerms(ctx context.Context, sess ports.Session, database, start string, limit int) ([]domain.TermInfo, error) {
	c := g.client(sess, database)
	terms, err := c.ReadTerms(ctx, start, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.TermInfo, 0, len(terms))
	for _, t := range terms {
		out = append(out, domain.TermInfo{Text: t.Text, Count: t.Count})
	}
	return out, nil
}

func (g *Gateway) GlobalCorrection(ctx context.Context, sess ports.Session, database string, req domain.GblRequest) (domain.GblResult, error) {
	c := g.client(sess, database)
	settings := &irbis.GblSettings{
		Database:         database,
		Actualize:        req.Actualize,
		Autoin:           req.Autoin,
		FormalControl:    req.FormalControl,
		Filename:         req.Filename,
		SearchExpression: req.SearchExpression,
		FirstRecord:      req.FirstRecord,
		NumberOfRecords:  req.NumberOfRecords,
		MinMfn:           req.MinMFN,
		MaxMfn:           req.MaxMFN,
		MfnList:          append([]int(nil), req.MfnList...),
		Statements:       make([]irbis.GblStatement, 0, len(req.Statements)),
	}
	for _, st := range req.Statements {
		settings.Statements = append(settings.Statements, irbis.GblStatement{
			Command:    st.Command,
			Parameter1: st.Parameter1,
			Parameter2: st.Parameter2,
			Format1:    st.Format1,
			Format2:    st.Format2,
		})
	}
	lines, err := c.GlobalCorrection(ctx, settings)
	if err != nil {
		return domain.GblResult{}, mapErr(err)
	}
	return domain.GblResult{Lines: lines}, nil
}

func (g *Gateway) ServerInfo(ctx context.Context, sess ports.Session) (domain.IrbisServerInfo, error) {
	c := g.client(sess, "")
	var info domain.IrbisServerInfo
	err := c.Do(ctx, func(conn *irbis.Connection) error {
		ver := conn.GetServerVersion()
		if e := conn.Err(); e != nil {
			return e
		}
		info.Version = ver.Version
		info.Organization = ver.Organization
		info.MaxClients = ver.MaxClients
		info.ConnectedClients = ver.ConnectedClients

		dbs := conn.ListDatabases(g.databasesMenu)
		if e := conn.Err(); e != nil {
			return e
		}
		info.Databases = make([]domain.DatabaseInfo, 0, len(dbs))
		for i := range dbs {
			info.Databases = append(info.Databases, domain.DatabaseInfo{
				Name:        dbs[i].Name,
				Description: dbs[i].Description,
				ReadOnly:    dbs[i].ReadOnly,
				MaxMFN:      dbs[i].MaxMfn,
			})
		}
		return nil
	})
	if err != nil {
		return domain.IrbisServerInfo{}, mapErr(err)
	}
	return info, nil
}
