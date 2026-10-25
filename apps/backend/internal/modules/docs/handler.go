package docs

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed assets/*
var assetsFS embed.FS

//go:embed openapi.yaml
var openapiYAML []byte

type Handler struct {
	assetServer http.Handler
}

func NewHandler() *Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return &Handler{
		assetServer: http.StripPrefix("/api/docs/assets/", http.FileServer(http.FS(sub))),
	}
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>CubyCloud API Documentation</title>
  <link rel="stylesheet" type="text/css" href="/api/docs/assets/swagger-ui.css" />
  <link rel="icon" type="image/png" href="/api/docs/assets/favicon-32x32.png" sizes="32x32" />
  <link rel="icon" type="image/png" href="/api/docs/assets/favicon-16x16.png" sizes="16x16" />
  <style>
    html {
      box-sizing: border-box;
      overflow: -moz-scrollbars-vertical;
      overflow-y: scroll;
    }
    *, *:before, *:after {
      box-sizing: inherit;
    }
    body {
      margin: 0;
      background: #fafafa;
    }
    .swagger-ui .topbar {
      display: none;
    }
    .swagger-ui .info {
      margin: 30px 0 20px 0;
    }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/api/docs/assets/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script src="/api/docs/assets/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/api/docs/openapi.yaml",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout",
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        docExpansion: "list",
        displayRequestDuration: true,
        persistAuthorization: true,
        requestInterceptor: function(req) {
          req.credentials = 'same-origin';
          return req;
        }
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`

func (h *Handler) GetDocsHTML(c echo.Context) error {
	return c.HTML(http.StatusOK, swaggerHTML)
}

func (h *Handler) GetOpenAPISpec(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "application/yaml; charset=utf-8")
	return c.Blob(http.StatusOK, "application/yaml; charset=utf-8", openapiYAML)
}

func (h *Handler) ServeAsset(c echo.Context) error {
	path := c.Param("*")
	if strings.Contains(path, "..") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid path")
	}
	h.assetServer.ServeHTTP(c.Response(), c.Request())
	return nil
}
