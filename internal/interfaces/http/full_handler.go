package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	appDocument "cmd/go-api/internal/application/document"
	appConfig "cmd/go-api/internal/config"
	"cmd/go-api/internal/infrastructure/stats"
)

// FullInvoiceBody merges the enriched QR parse result with the line-items table.
type FullInvoiceBody struct {
	Invoice *appDocument.ParseResult `json:"invoice" doc:"Dados da fatura descodificados do QR code, com nomes de emitente e adquirente identificados por IA (mesma estrutura de /api/v1/document/parse/enriched)"`
	Items   ItemsBody                `json:"items"   doc:"Linhas da tabela de itens extraídas por IA (apenas colunas e linhas)"`
}

// FullInvoiceOutput wraps FullInvoiceBody for Huma.
type FullInvoiceOutput struct {
	Body FullInvoiceBody
}

// FullInvoiceHandler runs the enriched QR parse and the items extractor in parallel,
// then returns a single combined JSON response.
func FullInvoiceHandler(service *appDocument.ScanService, cfg *appConfig.Config, counter *stats.Counter) func(context.Context, *ParseInput) (*FullInvoiceOutput, error) {
	return func(ctx context.Context, input *ParseInput) (*FullInvoiceOutput, error) {
		if cfg.ToolServerURL == "" {
			return nil, huma.Error503ServiceUnavailable("full invoice extraction requires TOOL_SERVER_URL to be configured")
		}

		pdfBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}

		type parseResult struct {
			result *appDocument.ParseResult
			err    error
		}
		type itemsResult struct {
			data []byte
			err  error
		}

		parseCh := make(chan parseResult, 1)
		itemsCh := make(chan itemsResult, 1)

		go func() {
			r, err := service.ParsePDF(pdfBytes)
			if err == nil {
				enrichParseResult(ctx, cfg, r)
			}
			parseCh <- parseResult{result: r, err: err}
		}()
		go func() {
			data, err := callToolServerUpload(ctx, cfg, "/tools/pdf/items/decode-upload", pdfBytes)
			itemsCh <- itemsResult{data: data, err: err}
		}()

		pr := <-parseCh
		ir := <-itemsCh

		if pr.err != nil {
			return nil, huma.Error422UnprocessableEntity("failed to process the PDF", pr.err)
		}
		if ir.err != nil {
			return nil, upstreamError("full.items", ir.err)
		}

		// items: only keep columns + rows.
		var itemsWrapper struct {
			Items struct {
				Columns []string                 `json:"columns"`
				Rows    []map[string]interface{} `json:"rows"`
			} `json:"items"`
		}
		if err := json.Unmarshal(ir.data, &itemsWrapper); err != nil {
			return nil, upstreamError("full.items.parse", err)
		}

		counter.Increment(sourceFromContext(ctx))

		return &FullInvoiceOutput{Body: FullInvoiceBody{
			Invoice: pr.result,
			Items: ItemsBody{
				Columns: itemsWrapper.Items.Columns,
				Rows:    itemsWrapper.Items.Rows,
			},
		}}, nil
	}
}

// callToolServerUpload posts a PDF as multipart/form-data to the given tool-server path.
// Uses the shared itemsClient (5-minute timeout).
func callToolServerUpload(ctx context.Context, cfg *appConfig.Config, path string, pdfBytes []byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "document.pdf")
	if err != nil {
		return nil, fmt.Errorf("creating multipart form: %w", err)
	}
	if _, err = part.Write(pdfBytes); err != nil {
		return nil, fmt.Errorf("writing PDF to form: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ToolServerURL+path, &body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cfg.ToolServerAPIKey != "" {
		req.Header.Set("x-api-key", cfg.ToolServerAPIKey)
	}

	resp, err := itemsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", path, resp.StatusCode, string(data))
	}
	return data, nil
}
