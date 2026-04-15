package http

import (
	"context"
	"fmt"
	"io"

	"github.com/danielgtaylor/huma/v2"

	appDocument "cmd/go-api/internal/application/document"
	appConfig "cmd/go-api/internal/config"
)

// ParsePDFEnrichedHandler parses a PDF and enriches emitente/adquirente with names from the NIF service.
func ParsePDFEnrichedHandler(service *appDocument.ScanService, cfg *appConfig.Config) func(context.Context, *ParseInput) (*ParseOutput, error) {
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
			return nil, huma.Error422UnprocessableEntity("failed to process the PDF", err)
		}

		enrichParseResult(ctx, cfg, result)
		return &ParseOutput{Body: result}, nil
	}
}

// ParseImageEnrichedHandler parses an image and enriches emitente/adquirente with names from the NIF service.
func ParseImageEnrichedHandler(service *appDocument.ScanService, cfg *appConfig.Config) func(context.Context, *ParseImageInput) (*ParseOutput, error) {
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
			return nil, huma.Error422UnprocessableEntity("failed to process the image", err)
		}

		enrichParseResult(ctx, cfg, result)
		return &ParseOutput{Body: result}, nil
	}
}

// enrichParseResult resolves NIF names and injects Descricao into every document in-place.
func enrichParseResult(ctx context.Context, cfg *appConfig.Config, result *appDocument.ParseResult) {
	// Collect unique NIFs across all documents.
	nifSet := make(map[string]struct{})
	for _, doc := range result.Documents {
		if doc.Emitente.NIF != "" {
			nifSet[doc.Emitente.NIF] = struct{}{}
		}
		if doc.Adquirente.NIF != "" {
			nifSet[doc.Adquirente.NIF] = struct{}{}
		}
	}

	if len(nifSet) == 0 {
		return
	}

	nifs := make([]string, 0, len(nifSet))
	for nif := range nifSet {
		nifs = append(nifs, nif)
	}

	lookup := resolveNIFsMap(ctx, cfg, nifs)

	for i := range result.Documents {
		if r, ok := lookup[result.Documents[i].Emitente.NIF]; ok && r.Found && r.Name != nil {
			result.Documents[i].Emitente.Descricao = *r.Name
		}
		if r, ok := lookup[result.Documents[i].Adquirente.NIF]; ok && r.Found && r.Name != nil {
			result.Documents[i].Adquirente.Descricao = *r.Name
		}
	}
}
