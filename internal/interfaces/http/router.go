package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	appDocument "cmd/go-api/internal/application/document"
	appConfig   "cmd/go-api/internal/config"
	"cmd/go-api/internal/ui"
)

// versionBody é o shape da resposta de GET /api/v1/version.
// Definido fora do handler para evitar conflito de tipos anónimos.
type versionBody struct {
	Version   string `json:"version"    doc:"Versão semântica da aplicação"`
	Author    string `json:"author"     doc:"Nome do autor"`
	AuthorURL string `json:"author_url" doc:"Website do autor"`
}

// NewRouter creates the Gin engine, wraps it with Huma, and registers all routes.
//
// Huma automatically:
//   - serves the OpenAPI 3.1 spec at  GET /openapi.json  and  /openapi.yaml
//   - serves the Swagger UI           at  GET /docs
//   - validates every request against the schema derived from the input structs
func NewRouter() *gin.Engine {
	router := gin.Default()
	router.MaxMultipartMemory = 32 << 20 // 32 MB max upload size

	// Huma config — this is all the "swagger" setup you need.
	humaConfig := huma.DefaultConfig("GoApi — Leitor de QR Code Fiscal ATCUD", appConfig.AppVersion)
	humaConfig.Info.Description = "Recebe um PDF, extrai todos os QR codes e devolve os que contêm um código ATCUD fiscal português."

	api := humagin.New(router, humaConfig)

	// GET / — interface web (HTML embutido no binário via go:embed).
	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", ui.IndexHTML)
	})

	// GET /health
	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Estado do serviço",
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body struct {
			Status string `json:"status" doc:"'ok' quando o serviço está operacional"`
		}
	}, error) {
		return &struct {
			Body struct {
				Status string `json:"status" doc:"'ok' quando o serviço está operacional"`
			}
		}{Body: struct {
			Status string `json:"status" doc:"'ok' quando o serviço está operacional"`
		}{Status: "ok"}}, nil
	})

	// GET /api/v1/version
	huma.Register(api, huma.Operation{
		OperationID: "version",
		Method:      http.MethodGet,
		Path:        "/api/v1/version",
		Summary:     "Versão da aplicação",
		Tags:        []string{"info"},
	}, func(ctx context.Context, _ *struct{}) (*struct{ Body versionBody }, error) {
		return &struct{ Body versionBody }{Body: versionBody{
			Version:   appConfig.AppVersion,
			Author:    appConfig.Author,
			AuthorURL: appConfig.AuthorURL,
		}}, nil
	})

	docService := appDocument.NewScanService()

	// POST /api/v1/document/scan
	huma.Register(api, huma.Operation{
		OperationID: "scan-pdf",
		Method:      http.MethodPost,
		Path:        "/api/v1/document/scan",
		Summary:     "Detectar QR codes ATCUD num PDF",
		Description: "Descodifica todos os QR codes em todas as páginas do PDF e devolve " +
			"apenas os que contêm um código ATCUD fiscal português válido.",
		Tags: []string{"documento"},
	}, ScanPDFHandler(docService))

	// POST /api/v1/document/parse
	huma.Register(api, huma.Operation{
		OperationID: "parse-pdf",
		Method:      http.MethodPost,
		Path:        "/api/v1/document/parse",
		Summary:     "Descodificar documento fiscal — dados estruturados",
		Description: "Extrai todos os QR codes ATCUD do PDF e descodifica cada um em campos " +
			"identificados: NIF do emitente, NIF do adquirente, tipo de documento, data, " +
			"linhas de IVA por taxa e região fiscal, total do documento e mais.",
		Tags: []string{"documento"},
	}, ParsePDFHandler(docService))

	return router
}
