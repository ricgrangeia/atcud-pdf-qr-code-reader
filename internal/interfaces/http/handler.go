// Package http wires HTTP concerns to the application layer.
// With Huma the handler is just a plain Go function — no annotations,
// no code-generation step. The OpenAPI 3.1 spec is derived automatically
// from the input/output structs you define here.
package http

import (
	"context"
	"fmt"
	"io"

	"github.com/danielgtaylor/huma/v2"

	appDocument "cmd/go-api/internal/application/document"
)

// -- Input ------------------------------------------------------------------

// scanFormData describes the multipart/form-data fields Huma expects.
// The `form` tag is the field name in the form, `contentType` restricts
// which MIME types are accepted, and `doc` becomes the field description
// in the generated spec.
type scanFormData struct {
	File huma.FormFile `form:"file" contentType:"application/pdf,application/octet-stream" doc:"PDF document to scan for ATCUD QR codes" required:"true"`
}

// ScanInput is the full request shape for POST /document/scan.
// Huma reads RawBody and maps every field in scanFormData from the form.
type ScanInput struct {
	RawBody huma.MultipartFormFiles[scanFormData]
}

// -- Output -----------------------------------------------------------------

// ScanOutput wraps the application result.
// Huma serialises Body as the JSON response body.
type ScanOutput struct {
	Body *appDocument.ScanResult
}

// -- Handler ----------------------------------------------------------------

// -- Parse handler ----------------------------------------------------------

// ParseInput is the request shape for POST /document/parse (same as ScanInput).
type ParseInput struct {
	RawBody huma.MultipartFormFiles[scanFormData]
}

// ParseOutput wraps the structured parse result.
type ParseOutput struct {
	Body *appDocument.ParseResult
}

// ParsePDFHandler returns the handler for POST /document/parse.
// It decodes every ATCUD QR code in the PDF into labelled fields:
// seller NIF, buyer NIF, document type, tax breakdown, gross total, etc.
func ParsePDFHandler(service *appDocument.ScanService) func(context.Context, *ParseInput) (*ParseOutput, error) {
	return func(ctx context.Context, input *ParseInput) (*ParseOutput, error) {
		formData := input.RawBody.Data()

		pdfBytes, err := io.ReadAll(formData.File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err),
			)
		}

		result, err := service.ParsePDF(pdfBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"failed to process the PDF", err,
			)
		}

		return &ParseOutput{Body: result}, nil
	}
}

// -- Image handlers ---------------------------------------------------------

// imageFormData describes the multipart/form-data fields for image uploads.
type imageFormData struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/gif,image/webp,image/tiff,application/octet-stream" doc:"Image containing QR code(s) — full page or cropped" required:"true"`
}

// ScanImageInput is the request shape for POST /image/scan.
type ScanImageInput struct {
	RawBody huma.MultipartFormFiles[imageFormData]
}

// ParseImageInput is the request shape for POST /image/parse.
type ParseImageInput struct {
	RawBody huma.MultipartFormFiles[imageFormData]
}

// ScanImageHandler returns the handler for POST /api/v1/image/scan.
func ScanImageHandler(service *appDocument.ScanService) func(context.Context, *ScanImageInput) (*ScanOutput, error) {
	return func(ctx context.Context, input *ScanImageInput) (*ScanOutput, error) {
		formData := input.RawBody.Data()

		imageBytes, err := io.ReadAll(formData.File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err),
			)
		}

		result, err := service.ScanImage(imageBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"failed to process the image", err,
			)
		}

		return &ScanOutput{Body: result}, nil
	}
}

// ParseImageHandler returns the handler for POST /api/v1/image/parse.
func ParseImageHandler(service *appDocument.ScanService) func(context.Context, *ParseImageInput) (*ParseOutput, error) {
	return func(ctx context.Context, input *ParseImageInput) (*ParseOutput, error) {
		formData := input.RawBody.Data()

		imageBytes, err := io.ReadAll(formData.File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err),
			)
		}

		result, err := service.ParseImage(imageBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"failed to process the image", err,
			)
		}

		return &ParseOutput{Body: result}, nil
	}
}

// -- Scan handler -----------------------------------------------------------

// ScanPDFHandler returns the handler function for POST /document/scan.
// It reads the uploaded PDF, hands it to the application service, and
// returns the QR codes that contain an ATCUD fiscal code.
//
// The function signature is all Huma needs to:
//   - generate the multipart/form-data request schema
//   - generate the JSON response schema
//   - validate the incoming request automatically
func ScanPDFHandler(service *appDocument.ScanService) func(context.Context, *ScanInput) (*ScanOutput, error) {
	return func(ctx context.Context, input *ScanInput) (*ScanOutput, error) {
		formData := input.RawBody.Data()

		// Read the uploaded file into memory.
		pdfBytes, err := io.ReadAll(formData.File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err),
			)
		}

		result, err := service.ScanPDF(pdfBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(
				"failed to process the PDF", err,
			)
		}

		return &ScanOutput{Body: result}, nil
	}
}
