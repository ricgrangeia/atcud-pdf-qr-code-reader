// Package http wires HTTP concerns to the application layer.
package http

import (
	"context"
	"fmt"
	"io"

	"github.com/danielgtaylor/huma/v2"

	appDocument "cmd/go-api/internal/application/document"
	"cmd/go-api/internal/infrastructure/stats"
)

// -- Input types ------------------------------------------------------------

type scanFormData struct {
	File huma.FormFile `form:"file" contentType:"application/pdf,application/octet-stream" doc:"PDF document to scan for ATCUD QR codes" required:"true"`
}

type ScanInput struct {
	RawBody huma.MultipartFormFiles[scanFormData]
}

type ScanOutput struct {
	Body *appDocument.ScanResult
}

type ParseInput struct {
	RawBody huma.MultipartFormFiles[scanFormData]
}

type ParseOutput struct {
	Body *appDocument.ParseResult
}

type imageFormData struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/gif,image/webp,image/tiff,application/octet-stream" doc:"Image containing QR code(s) — full page or cropped" required:"true"`
}

type ScanImageInput struct {
	RawBody huma.MultipartFormFiles[imageFormData]
}

type ParseImageInput struct {
	RawBody huma.MultipartFormFiles[imageFormData]
}

// -- PDF handlers -----------------------------------------------------------

func ScanPDFHandler(service *appDocument.ScanService, counter *stats.Counter) func(context.Context, *ScanInput) (*ScanOutput, error) {
	return func(ctx context.Context, input *ScanInput) (*ScanOutput, error) {
		pdfBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}
		result, err := service.ScanPDF(pdfBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("failed to process the PDF", err)
		}
		counter.Increment()
		return &ScanOutput{Body: result}, nil
	}
}

func ParsePDFHandler(service *appDocument.ScanService, counter *stats.Counter) func(context.Context, *ParseInput) (*ParseOutput, error) {
	return func(ctx context.Context, input *ParseInput) (*ParseOutput, error) {
		pdfBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}
		result, err := service.ParsePDF(pdfBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("failed to process the PDF", err)
		}
		counter.Increment()
		return &ParseOutput{Body: result}, nil
	}
}

// -- Image handlers ---------------------------------------------------------

func ScanImageHandler(service *appDocument.ScanService, counter *stats.Counter) func(context.Context, *ScanImageInput) (*ScanOutput, error) {
	return func(ctx context.Context, input *ScanImageInput) (*ScanOutput, error) {
		imageBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}
		result, err := service.ScanImage(imageBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("failed to process the image", err)
		}
		counter.Increment()
		return &ScanOutput{Body: result}, nil
	}
}

func ParseImageHandler(service *appDocument.ScanService, counter *stats.Counter) func(context.Context, *ParseImageInput) (*ParseOutput, error) {
	return func(ctx context.Context, input *ParseImageInput) (*ParseOutput, error) {
		imageBytes, err := io.ReadAll(input.RawBody.Data().File)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("could not read the uploaded file", fmt.Errorf("io.ReadAll: %w", err))
		}
		result, err := service.ParseImage(imageBytes)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("failed to process the image", err)
		}
		counter.Increment()
		return &ParseOutput{Body: result}, nil
	}
}
