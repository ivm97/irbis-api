package httpapi

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/usecase"
)

// Handler exposes REST endpoints backed by CatalogService.
type Handler struct {
	svc *usecase.CatalogService
}

func NewHandler(svc *usecase.CatalogService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(c echo.Context) error {
	user, pass := credentials(c)
	res, err := h.svc.Me(c.Request().Context(), user, pass)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) serverConfig(c echo.Context) error {
	user, pass := credentials(c)
	res, err := h.svc.ServerConfig(c.Request().Context(), user, pass, workstation(c))
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func parseSearchQuery(c echo.Context) domain.SearchQuery {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	withRecords, _ := strconv.ParseBool(c.QueryParam("with_records"))
	return domain.SearchQuery{
		Expression:  c.QueryParam("expression"),
		Mode:        domain.SearchMode(c.QueryParam("mode")),
		Value:       c.QueryParam("value"),
		Limit:       limit,
		Offset:      offset,
		Format:      c.QueryParam("format"),
		WithRecords: withRecords,
	}
}

type searchBody struct {
	Expression  string            `json:"expression"`
	Mode        domain.SearchMode `json:"mode"`
	Value       string            `json:"value"`
	Limit       int               `json:"limit"`
	Offset      int               `json:"offset"`
	Format      string            `json:"format"`
	WithRecords bool              `json:"with_records"`
	MinMFN      int               `json:"min_mfn"`
	MaxMFN      int               `json:"max_mfn"`
	Sequential  string            `json:"sequential"`
}

func (h *Handler) searchGET(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	q := parseSearchQuery(c)
	res, err := h.svc.Search(c.Request().Context(), user, pass, workstation(c), db, q)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) searchPOST(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	var body searchBody
	if err := c.Bind(&body); err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	q := domain.SearchQuery{
		Expression:  body.Expression,
		Mode:        body.Mode,
		Value:       body.Value,
		Limit:       body.Limit,
		Offset:      body.Offset,
		Format:      body.Format,
		WithRecords: body.WithRecords,
		MinMFN:      body.MinMFN,
		MaxMFN:      body.MaxMFN,
		Sequential:  body.Sequential,
	}
	res, err := h.svc.Search(c.Request().Context(), user, pass, workstation(c), db, q)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) count(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	q := parseSearchQuery(c)
	expr, total, err := h.svc.Count(c.Request().Context(), user, pass, workstation(c), db, q)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"expression": expr,
		"total":      total,
	})
}

func (h *Handler) getRecord(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	mfn, err := strconv.Atoi(c.Param("mfn"))
	if err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	rec, err := h.svc.GetRecord(c.Request().Context(), user, pass, workstation(c), db, mfn, c.QueryParam("format"))
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, rec)
}

func (h *Handler) createRecord(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	var body domain.RecordWrite
	if err := c.Bind(&body); err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	body.MFN = 0
	rec, err := h.svc.SaveRecord(c.Request().Context(), user, pass, workstation(c), db, body)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusCreated, rec)
}

func (h *Handler) putRecord(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	mfn, err := strconv.Atoi(c.Param("mfn"))
	if err != nil || mfn <= 0 {
		return writeErr(c, domain.ErrInvalidInput)
	}
	var body domain.RecordWrite
	if err := c.Bind(&body); err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	body.MFN = mfn
	rec, err := h.svc.SaveRecord(c.Request().Context(), user, pass, workstation(c), db, body)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, rec)
}

func (h *Handler) deleteRecord(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	mfn, err := strconv.Atoi(c.Param("mfn"))
	if err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	if err := h.svc.DeleteRecord(c.Request().Context(), user, pass, workstation(c), db, mfn); err != nil {
		return writeErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

type batchBody struct {
	Records []domain.RecordWrite `json:"records"`
}

func (h *Handler) batchRecords(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	var body batchBody
	if err := c.Bind(&body); err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	recs, err := h.svc.SaveRecords(c.Request().Context(), user, pass, workstation(c), db, body.Records)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"records": recs})
}

func (h *Handler) globalCorrection(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	var body domain.GblRequest
	if err := c.Bind(&body); err != nil {
		return writeErr(c, domain.ErrInvalidInput)
	}
	res, err := h.svc.GlobalCorrection(c.Request().Context(), user, pass, workstation(c), db, body)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) listTerms(c echo.Context) error {
	user, pass := credentials(c)
	db := c.Param("database")
	start := c.QueryParam("start")
	if start == "" {
		return writeErr(c, domain.ErrInvalidInput)
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	terms, err := h.svc.ListTerms(c.Request().Context(), user, pass, workstation(c), db, start, limit)
	if err != nil {
		return writeErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"terms": terms})
}
