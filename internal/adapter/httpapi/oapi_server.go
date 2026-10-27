package httpapi

import (
	"github.com/labstack/echo/v4"

	apigen "github.com/ivm97/irbis-api/api/gen"
)

// Ensure Handler implements the generated Echo server interface.
var _ apigen.ServerInterface = (*Handler)(nil)

// Register mounts generated OpenAPI routes (echo.Context) plus Swagger UI.
func (h *Handler) Register(e *echo.Echo) {
	apigen.RegisterHandlers(e, h)
	RegisterSwagger(e)
}

func (h *Handler) GetHealth(ctx echo.Context) error {
	return h.health(ctx)
}

func (h *Handler) Login(ctx echo.Context) error {
	return h.login(ctx)
}

func (h *Handler) Logout(ctx echo.Context) error {
	return h.logout(ctx)
}

func (h *Handler) GetMe(ctx echo.Context, _ apigen.GetMeParams) error {
	return h.me(ctx)
}

func (h *Handler) GetConfig(ctx echo.Context, _ apigen.GetConfigParams) error {
	return h.serverConfig(ctx)
}

func (h *Handler) SearchRecords(ctx echo.Context, _ apigen.Database, _ apigen.SearchRecordsParams) error {
	return h.searchGET(ctx)
}

func (h *Handler) SearchRecordsAdvanced(ctx echo.Context, _ apigen.Database, _ apigen.SearchRecordsAdvancedParams) error {
	return h.searchPOST(ctx)
}

func (h *Handler) CountRecords(ctx echo.Context, _ apigen.Database, _ apigen.CountRecordsParams) error {
	return h.count(ctx)
}

func (h *Handler) GetRecord(ctx echo.Context, _ apigen.Database, _ apigen.Mfn, _ apigen.GetRecordParams) error {
	return h.getRecord(ctx)
}

func (h *Handler) CreateRecord(ctx echo.Context, _ apigen.Database, _ apigen.CreateRecordParams) error {
	return h.createRecord(ctx)
}

func (h *Handler) PutRecord(ctx echo.Context, _ apigen.Database, _ apigen.Mfn, _ apigen.PutRecordParams) error {
	return h.putRecord(ctx)
}

func (h *Handler) DeleteRecord(ctx echo.Context, _ apigen.Database, _ apigen.Mfn, _ apigen.DeleteRecordParams) error {
	return h.deleteRecord(ctx)
}

func (h *Handler) PutRecordsBatch(ctx echo.Context, _ apigen.Database, _ apigen.PutRecordsBatchParams) error {
	return h.batchRecords(ctx)
}

func (h *Handler) RunGlobalCorrection(ctx echo.Context, _ apigen.Database, _ apigen.RunGlobalCorrectionParams) error {
	return h.globalCorrection(ctx)
}

func (h *Handler) ListTerms(ctx echo.Context, _ apigen.Database, _ apigen.ListTermsParams) error {
	return h.listTerms(ctx)
}
