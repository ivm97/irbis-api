package ports

import (
	"context"

	"github.com/ivm97/irbis-api/internal/domain"
)

// AuthStore validates API credentials and returns the user principal.
type AuthStore interface {
	Authenticate(ctx context.Context, username, password string) (domain.User, error)
}

// Session carries IRBIS TCP credentials and ARM for one request.
type Session struct {
	Username    string
	Password    string
	Workstation domain.Workstation
}

// Catalog is the IRBIS gateway used by use-cases.
type Catalog interface {
	Search(ctx context.Context, sess Session, database string, q domain.SearchQuery) (domain.SearchResult, error)
	Count(ctx context.Context, sess Session, database string, q domain.SearchQuery) (expression string, total int, err error)
	GetRecord(ctx context.Context, sess Session, database string, mfn int, format string) (domain.Record, error)
	GetRecords(ctx context.Context, sess Session, database string, mfns []int) ([]domain.Record, error)
	SaveRecord(ctx context.Context, sess Session, database string, in domain.RecordWrite) (domain.Record, error)
	SaveRecords(ctx context.Context, sess Session, database string, in []domain.RecordWrite) ([]domain.Record, error)
	DeleteRecord(ctx context.Context, sess Session, database string, mfn int) error
	ListTerms(ctx context.Context, sess Session, database, start string, limit int) ([]domain.TermInfo, error)
	GlobalCorrection(ctx context.Context, sess Session, database string, req domain.GblRequest) (domain.GblResult, error)
	ServerInfo(ctx context.Context, sess Session) (domain.IrbisServerInfo, error)
}
