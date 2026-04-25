package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	appConfig "cmd/go-api/internal/config"
	"cmd/go-api/internal/infrastructure/stats"
)

// ItemsBody is the response body of /api/v1/document/items.
// Only columns + rows are returned — currency/totals come from the QR code endpoints.
type ItemsBody struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

// ItemsOutput wraps ItemsBody for Huma.
type ItemsOutput struct {
	Body ItemsBody
}

var itemsClient = &http.Client{Timeout: 300 * time.Second}

// ItemsHandler proxies a PDF upload to the tool server's items extractor and returns
// only the structured line items (columns + rows). Totals/currency are intentionally
// dropped — they belong to the QR code parse endpoints.
func ItemsHandler(cfg *appConfig.Config, counter *stats.Counter) func(context.Context, *ParseInput) (*ItemsOutput, error) {
	return func(ctx context.Context, input *ParseInput) (*ItemsOutput, error) {
		if cfg.ToolServerURL == "" {
			return nil, huma.Error503ServiceUnavailable("items extraction requires TOOL_SERVER_URL to be configured")
		}

		pdfBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}

		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file", "document.pdf")
		if err != nil {
			return nil, internalError("items.multipart", err)
		}
		if _, err = part.Write(pdfBytes); err != nil {
			return nil, internalError("items.multipart", err)
		}
		w.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			cfg.ToolServerURL+"/tools/pdf/items/decode-upload", &body)
		if err != nil {
			return nil, internalError("items.request", err)
		}
		req.Header.Set("Content-Type", w.FormDataContentType())
		if cfg.ToolServerAPIKey != "" {
			req.Header.Set("x-api-key", cfg.ToolServerAPIKey)
		}

		resp, err := itemsClient.Do(req)
		if err != nil {
			return nil, upstreamError("items.call", err)
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, upstreamError("items.read", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, upstreamError("items.status",
				fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw)))
		}

		// Tool server response shape:
		//   { "filename": "...", "items": { "columns": [...], "rows": [...], ... } }
		var wrapper struct {
			Items struct {
				Columns []string                 `json:"columns"`
				Rows    []map[string]interface{} `json:"rows"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			return nil, upstreamError("items.parse", err)
		}

		counter.Increment(sourceFromContext(ctx))

		return &ItemsOutput{Body: ItemsBody{
			Columns: wrapper.Items.Columns,
			Rows:    wrapper.Items.Rows,
		}}, nil
	}
}
