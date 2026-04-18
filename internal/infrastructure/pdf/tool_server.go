package pdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

var toolServerClient = &http.Client{Timeout: 300 * time.Second}

// ScanPDFViaToolServer forwards a PDF to the tool server's /tools/pdf/qr/decode endpoint
// and returns the decoded QR code strings. The response schema is unspecified ({}) so
// this function handles common patterns: array of strings, array of objects with a
// "content" field, or an object wrapping such an array.
func ScanPDFViaToolServer(pdfBytes []byte, toolServerURL, apiKey string) ([]RawQRCode, error) {
	url := toolServerURL + "/tools/pdf/qr/decode"

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "document.pdf")
	if err != nil {
		return nil, fmt.Errorf("creating multipart field: %w", err)
	}
	if _, err = part.Write(pdfBytes); err != nil {
		return nil, fmt.Errorf("writing PDF bytes: %w", err)
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := toolServerClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling tool server: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tool server returned HTTP %d: %s", resp.StatusCode, string(data))
	}

	return parseToolServerResponse(data)
}

// parseToolServerResponse extracts QR code strings from the tool server response.
// It handles several common shapes since the endpoint schema is unspecified.
func parseToolServerResponse(data []byte) ([]RawQRCode, error) {
	// Shape 1: array of strings — ["A:NIF*...", ...]
	var strSlice []string
	if err := json.Unmarshal(data, &strSlice); err == nil {
		return stringsToRawQRCodes(strSlice), nil
	}

	// Shape 2: array of objects — [{"content":"...", "page":1, "method":"..."}, ...]
	var objSlice []map[string]interface{}
	if err := json.Unmarshal(data, &objSlice); err == nil {
		return objectsToRawQRCodes(objSlice), nil
	}

	// Shape 3: wrapped object — {"qrcodes": [...], ...}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err == nil {
		for _, key := range []string{"qrcodes", "qr_codes", "results", "codes", "data"} {
			if raw, ok := wrapper[key]; ok {
				if inner, err := json.Marshal(raw); err == nil {
					if results, err := parseToolServerResponse(inner); err == nil {
						return results, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("unrecognised tool server response format: %s", string(data))
}

func stringsToRawQRCodes(ss []string) []RawQRCode {
	out := make([]RawQRCode, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, RawQRCode{Content: s, PageNumber: 1})
		}
	}
	return out
}

func objectsToRawQRCodes(objs []map[string]interface{}) []RawQRCode {
	out := make([]RawQRCode, 0, len(objs))
	for _, obj := range objs {
		content := ""
		for _, key := range []string{"data", "content", "text", "qr_content"} {
			if v, ok := obj[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					content = s
					break
				}
			}
		}
		if content == "" {
			continue
		}
		page := 1
		if v, ok := obj["page"]; ok {
			if n, ok := v.(float64); ok {
				page = int(n)
			}
		}
		out = append(out, RawQRCode{Content: content, PageNumber: page})
	}
	return out
}
