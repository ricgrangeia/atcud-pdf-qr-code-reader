package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	appDocument "cmd/go-api/internal/application/document"
	appConfig   "cmd/go-api/internal/config"
	"cmd/go-api/internal/infrastructure/stats"
	"cmd/go-api/internal/ui"
)

// versionBody é o shape da resposta de GET /api/v1/version.
type versionBody struct {
	Version   string `json:"version"    doc:"Versão semântica da aplicação"`
	Author    string `json:"author"     doc:"Nome do autor"`
	AuthorURL string `json:"author_url" doc:"Website do autor"`
}

// statsBody é o shape da resposta de GET /api/v1/stats.
type statsBody struct {
	Total          int64            `json:"total"             doc:"Total de documentos analisados desde sempre"`
	ThisMonth      int64            `json:"this_month"        doc:"Documentos analisados no mês corrente"`
	ThisMonthWeb   int64            `json:"this_month_web"    doc:"Documentos analisados no mês corrente via página web"`
	ThisMonthApp   int64            `json:"this_month_app"    doc:"Documentos analisados no mês corrente via app Android"`
	ThisMonthOther int64            `json:"this_month_other"  doc:"Documentos analisados no mês corrente por outros clientes (API directa)"`
	Month          string           `json:"month"             doc:"Mês corrente no formato AAAA-MM"`
	Sources        map[string]int64 `json:"sources"           doc:"Totais históricos por cliente (web, android, api)"`
}

// NewRouter creates the Gin engine, wraps it with Huma, and registers all routes.
func NewRouter(cfg *appConfig.Config, counter *stats.Counter) *gin.Engine {
	router := gin.Default()
	router.SetTrustedProxies([]string{"172.16.0.0/12"}) // Docker bridge range — covers the Traefik proxy network
	router.MaxMultipartMemory = 32 << 20                 // 32 MB max upload size
	router.Use(clientSourceMiddleware())

	humaConfig := huma.DefaultConfig("GoApi — Leitor de QR Code Fiscal ATCUD", appConfig.AppVersion)
	humaConfig.Info.Description = "Recebe um PDF ou imagem, extrai todos os QR codes e devolve os que contêm um código ATCUD fiscal português."

	api := humagin.New(router, humaConfig)

	// GET / — interface web embutida no binário via go:embed.
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

	// GET /api/v1/stats
	huma.Register(api, huma.Operation{
		OperationID: "stats",
		Method:      http.MethodGet,
		Path:        "/api/v1/stats",
		Summary:     "Estatísticas de utilização",
		Tags:        []string{"info"},
	}, func(ctx context.Context, _ *struct{}) (*struct{ Body statsBody }, error) {
		s := counter.Stats()
		return &struct{ Body statsBody }{Body: statsBody{
			Total:          s.Total,
			ThisMonth:      s.ThisMonth,
			ThisMonthWeb:   s.ThisMonthWeb,
			ThisMonthApp:   s.ThisMonthApp,
			ThisMonthOther: s.ThisMonthOther,
			Month:          s.Month,
			Sources:        s.Sources,
		}}, nil
	})

	docService := appDocument.NewScanService(cfg)

	// POST /api/v1/document/scan
	huma.Register(api, huma.Operation{
		OperationID: "scan-pdf",
		Method:      http.MethodPost,
		Path:        "/api/v1/document/scan",
		Summary:     "Detectar QR codes ATCUD num PDF",
		Description: "Descodifica todos os QR codes em todas as páginas do PDF e devolve " +
			"apenas os que contêm um código ATCUD fiscal português válido.",
		Tags: []string{"documento"},
	}, ScanPDFHandler(docService, counter))

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
	}, ParsePDFHandler(docService, counter))

	// POST /api/v1/image/scan
	huma.Register(api, huma.Operation{
		OperationID: "scan-image",
		Method:      http.MethodPost,
		Path:        "/api/v1/image/scan",
		Summary:     "Detectar QR codes ATCUD numa imagem",
		Description: "Recebe uma imagem (JPEG, PNG, GIF, WEBP, TIFF) — página completa ou recorte — " +
			"e devolve o conteúdo bruto dos QR codes que contêm um código ATCUD fiscal português válido.",
		Tags: []string{"imagem"},
	}, ScanImageHandler(docService, counter))

	// POST /api/v1/image/parse
	huma.Register(api, huma.Operation{
		OperationID: "parse-image",
		Method:      http.MethodPost,
		Path:        "/api/v1/image/parse",
		Summary:     "Descodificar imagem fiscal — dados estruturados",
		Description: "Recebe uma imagem (JPEG, PNG, GIF, WEBP, TIFF) — página completa ou recorte — " +
			"e descodifica cada QR code ATCUD em campos identificados: NIF do emitente, NIF do adquirente, " +
			"tipo de documento, data, linhas de IVA por taxa e região fiscal, total do documento e mais.",
		Tags: []string{"imagem"},
	}, ParseImageHandler(docService, counter))

	// POST /api/v1/document/parse/enriched
	huma.Register(api, huma.Operation{
		OperationID: "parse-pdf-enriched",
		Method:      http.MethodPost,
		Path:        "/api/v1/document/parse/enriched",
		Summary:     "Descodificar PDF com identificação de entidades por IA",
		Description: "Igual a /document/parse mas inclui o nome da entidade (emitente e adquirente) " +
			"resolvido pelo serviço de NIF nos campos `descricao`.",
		Tags: []string{"documento"},
	}, ParsePDFEnrichedHandler(docService, cfg, counter))

	// POST /api/v1/image/parse/enriched
	huma.Register(api, huma.Operation{
		OperationID: "parse-image-enriched",
		Method:      http.MethodPost,
		Path:        "/api/v1/image/parse/enriched",
		Summary:     "Descodificar imagem com identificação de entidades por IA",
		Description: "Igual a /image/parse mas inclui o nome da entidade (emitente e adquirente) " +
			"resolvido pelo serviço de NIF nos campos `descricao`.",
		Tags: []string{"imagem"},
	}, ParseImageEnrichedHandler(docService, cfg, counter))

	// POST /api/v1/document/items
	huma.Register(api, huma.Operation{
		OperationID: "items-pdf",
		Method:      http.MethodPost,
		Path:        "/api/v1/document/items",
		Summary:     "Extrair linhas de itens de um PDF",
		Description: "Envia o PDF ao serviço externo de extracção de tabelas e devolve apenas " +
			"as colunas e linhas detectadas. Totais e moeda devem ser obtidos pelos endpoints " +
			"de descodificação do QR code (/document/parse).",
		Tags: []string{"documento"},
	}, ItemsHandler(cfg, counter))

	// POST /api/v1/nif/lookup/bulk
	huma.Register(api, huma.Operation{
		OperationID: "nif-lookup-bulk",
		Method:      http.MethodPost,
		Path:        "/api/v1/nif/lookup/bulk",
		Summary:     "Resolver NIFs portugueses para nomes",
		Description: "Recebe uma lista de NIFs e devolve o nome da entidade, actividade e morada " +
			"para cada um. NIFs especiais (999999990, 999999999) são resolvidos localmente. " +
			"Os restantes são consultados no serviço externo configurado em TOOL_SERVER_URL.",
		Tags: []string{"nif"},
	}, NifBulkHandler(cfg))

	return router
}
