package httpapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ivm97/irbis-api/internal/domain"
)

type errorBody struct {
	Error string `json:"error"`
}

func writeErr(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return c.JSON(http.StatusUnauthorized, errorBody{Error: "unauthorized"})
	case errors.Is(err, domain.ErrForbidden):
		return c.JSON(http.StatusForbidden, errorBody{Error: err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		return c.JSON(http.StatusNotFound, errorBody{Error: "not found"})
	case errors.Is(err, domain.ErrInvalidInput):
		return c.JSON(http.StatusBadRequest, errorBody{Error: err.Error()})
	case errors.Is(err, domain.ErrIrbis):
		return c.JSON(http.StatusBadGateway, errorBody{Error: err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, errorBody{Error: err.Error()})
	}
}
