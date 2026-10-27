package httpapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// RegisterSwagger mounts OpenAPI spec and a minimal Swagger UI.
// Spec path can be overridden with OPENAPI_PATH (default api/openapi.yaml).
func RegisterSwagger(e *echo.Echo) {
	specPath := os.Getenv("OPENAPI_PATH")
	if specPath == "" {
		specPath = "api/openapi.yaml"
	}

	e.GET("/openapi.yaml", func(c echo.Context) error {
		return c.File(specPath)
	})
	e.GET("/swagger", func(c echo.Context) error {
		return c.HTML(http.StatusOK, swaggerHTML)
	})
	e.GET("/swagger/", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/swagger")
	})
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Irbis API — Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  window.ui = SwaggerUIBundle({
    url: '/openapi.yaml',
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis],
    layout: 'BaseLayout'
  });
</script>
</body>
</html>
`
