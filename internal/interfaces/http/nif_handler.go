package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	appConfig "cmd/go-api/internal/config"
)

// NifResult mirrors the NifResult schema from the AI tool server.
type NifResult struct {
	NIF       string  `json:"nif"`
	Found     bool    `json:"found"`
	Name      *string `json:"name,omitempty"`
	Activity  *string `json:"activity,omitempty"`
	CAE       *string `json:"cae,omitempty"`
	Address   *string `json:"address,omitempty"`
	Situation *string `json:"situation,omitempty"`
	Error     *string `json:"error,omitempty"`
}

// NifBulkInput is the request shape for POST /api/v1/nif/lookup/bulk.
type NifBulkInput struct {
	Body struct {
		NIFs []string `json:"nifs" doc:"NIFs portugueses a resolver (máx. 20)" minItems:"1" maxItems:"20"`
	}
}

// NifBulkOutput wraps the bulk lookup results.
type NifBulkOutput struct {
	Body []NifResult
}

// specialNIFs are resolved locally without calling the external service.
var specialNIFs = map[string]string{
	"999999990": "Consumidor Final",
	"999999999": "Sujeito Passivo não residente sem NIF",
}

// NifBulkHandler returns the handler for POST /api/v1/nif/lookup/bulk.
// It resolves special NIFs locally and proxies the rest to the AI tool server.
func NifBulkHandler(cfg *appConfig.Config) func(context.Context, *NifBulkInput) (*NifBulkOutput, error) {
	return func(ctx context.Context, input *NifBulkInput) (*NifBulkOutput, error) {
		results := make([]NifResult, 0, len(input.Body.NIFs))
		var toLookup []string

		for _, nif := range input.Body.NIFs {
			if name, ok := specialNIFs[nif]; ok {
				n := name
				results = append(results, NifResult{NIF: nif, Found: true, Name: &n})
			} else {
				toLookup = append(toLookup, nif)
			}
		}

		if len(toLookup) == 0 {
			return &NifBulkOutput{Body: results}, nil
		}

		if cfg.ToolServerURL == "" {
			msg := "serviço de consulta de NIF não configurado (TOOL_SERVER_URL)"
			for _, nif := range toLookup {
				m := msg
				results = append(results, NifResult{NIF: nif, Found: false, Error: &m})
			}
			return &NifBulkOutput{Body: results}, nil
		}

		external, err := proxyNIFBulk(ctx, cfg, toLookup)
		if err != nil {
			return nil, huma.Error502BadGateway("falha ao consultar serviço de NIF", err)
		}

		results = append(results, external...)
		return &NifBulkOutput{Body: results}, nil
	}
}

// resolveNIFsMap resolves a slice of NIFs (special + external) and returns a map nif→NifResult.
// Never returns an error — failures for individual NIFs are stored inside the result.
func resolveNIFsMap(ctx context.Context, cfg *appConfig.Config, nifs []string) map[string]NifResult {
	out := make(map[string]NifResult, len(nifs))
	var toLookup []string

	for _, nif := range nifs {
		if name, ok := specialNIFs[nif]; ok {
			n := name
			out[nif] = NifResult{NIF: nif, Found: true, Name: &n}
		} else {
			toLookup = append(toLookup, nif)
		}
	}

	if len(toLookup) > 0 && cfg.ToolServerURL != "" {
		if results, err := proxyNIFBulk(ctx, cfg, toLookup); err == nil {
			for _, r := range results {
				out[r.NIF] = r
			}
		}
	}

	return out
}

func proxyNIFBulk(ctx context.Context, cfg *appConfig.Config, nifs []string) ([]NifResult, error) {
	body, err := json.Marshal(map[string][]string{"nifs": nifs})
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ToolServerURL+"/tools/nif/lookup/bulk",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if cfg.ToolServerAPIKey != "" {
		req.Header.Set("x-api-key", cfg.ToolServerAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling NIF lookup service: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NIF lookup service returned HTTP %d: %s", resp.StatusCode, string(data))
	}

	var results []NifResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return results, nil
}
