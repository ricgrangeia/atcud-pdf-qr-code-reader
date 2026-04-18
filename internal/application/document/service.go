// Package document contains application-level use cases for fiscal document processing.
package document

import (
	"fmt"

	appConfig "cmd/go-api/internal/config"
	domain "cmd/go-api/internal/domain/document"
	"cmd/go-api/internal/infrastructure/pdf"
)

// ScanResult is what the API caller receives after a PDF is processed.
type ScanResult struct {
	// TotalQRCodes is the number of QR codes found across all pages.
	TotalQRCodes int `json:"total_qr_codes"`

	// ATCUDCount is how many of those QR codes contained an ATCUD code.
	ATCUDCount int `json:"atcud_count"`

	// QRCodes holds only the QR codes that contain a valid ATCUD.
	QRCodes []domain.QRCode `json:"qr_codes"`
}

// ParseResult is returned by ParsePDF: every ATCUD QR code fully decoded
// into labelled, human-readable fields.
type ParseResult struct {
	// TotalQRCodes is the total number of QR codes found across all pages.
	TotalQRCodes int `json:"total_qr_codes"`

	// ParsedCount is how many of those contained a valid ATCUD and were parsed.
	ParsedCount int `json:"parsed_count"`

	// Documents holds the structured data for each parsed QR code.
	Documents []domain.ParsedQRCode `json:"documents"`
}

// ScanService is the use-case handler for scanning a PDF for ATCUD QR codes.
type ScanService struct {
	cfg *appConfig.Config
}

// NewScanService creates a ScanService.
func NewScanService(cfg *appConfig.Config) *ScanService {
	return &ScanService{cfg: cfg}
}

// extractFromPDF returns raw QR codes from a PDF.
// It uses the tool server when configured, falling back to local scanning.
func (s *ScanService) extractFromPDF(pdfBytes []byte) ([]pdf.RawQRCode, error) {
	if s.cfg != nil && s.cfg.ToolServerURL != "" {
		raw, err := pdf.ScanPDFViaToolServer(pdfBytes, s.cfg.ToolServerURL, s.cfg.ToolServerAPIKey)
		if err == nil {
			return raw, nil
		}
		// Tool server failed — fall through to local scanning.
	}
	return pdf.ExtractQRCodes(pdfBytes)
}

// ScanPDF reads a PDF, extracts every QR code, and returns those that carry an ATCUD.
//
// Steps:
//  1. Hand the raw PDF bytes to the infrastructure scanner.
//  2. For each decoded QR code, ask the domain whether it contains an ATCUD.
//  3. Collect and return only the ones that do.
func (s *ScanService) ScanPDF(pdfBytes []byte) (*ScanResult, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("the PDF file is empty")
	}

	raw, err := s.extractFromPDF(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting QR codes from PDF: %w", err)
	}

	result := &ScanResult{
		TotalQRCodes: len(raw),
		QRCodes:      make([]domain.QRCode, 0),
	}

	seen := make(map[string]struct{})
	for _, r := range raw {
		atcud, hasATCUD := domain.DetectATCUD(r.Content)
		if !hasATCUD {
			continue
		}
		if _, dup := seen[atcud]; dup {
			continue
		}
		seen[atcud] = struct{}{}
		result.QRCodes = append(result.QRCodes, domain.QRCode{
			Content:    r.Content,
			PageNumber: r.PageNumber,
			HasATCUD:   true,
			ATCUD:      atcud,
		})
	}

	result.ATCUDCount = len(result.QRCodes)
	return result, nil
}

// ScanImage reads an image, extracts every QR code, and returns those that carry an ATCUD.
func (s *ScanService) ScanImage(imageBytes []byte) (*ScanResult, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("the image file is empty")
	}

	raw, err := pdf.ExtractQRCodesFromImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting QR codes from image: %w", err)
	}

	result := &ScanResult{
		TotalQRCodes: len(raw),
		QRCodes:      make([]domain.QRCode, 0),
	}

	for _, r := range raw {
		atcud, hasATCUD := domain.DetectATCUD(r.Content)
		if hasATCUD {
			result.QRCodes = append(result.QRCodes, domain.QRCode{
				Content:    r.Content,
				PageNumber: r.PageNumber,
				HasATCUD:   true,
				ATCUD:      atcud,
			})
		}
	}

	result.ATCUDCount = len(result.QRCodes)
	return result, nil
}

// ParseImage extracts every ATCUD QR code from the image and returns each one
// fully decoded into structured, human-readable fields.
func (s *ScanService) ParseImage(imageBytes []byte) (*ParseResult, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("the image file is empty")
	}

	raw, err := pdf.ExtractQRCodesFromImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting QR codes from image: %w", err)
	}

	result := &ParseResult{
		TotalQRCodes: len(raw),
		Documents:    make([]domain.ParsedQRCode, 0),
	}

	for _, r := range raw {
		if _, hasATCUD := domain.DetectATCUD(r.Content); !hasATCUD {
			continue
		}

		parsed, err := domain.ParseQRCode(r.Content)
		if err != nil {
			result.Documents = append(result.Documents, domain.ParsedQRCode{
				NumeroPagina:  r.PageNumber,
				ConteudoBruto: r.Content,
			})
			continue
		}

		parsed.NumeroPagina = r.PageNumber
		result.Documents = append(result.Documents, *parsed)
	}

	result.ParsedCount = len(result.Documents)
	return result, nil
}

// ParsePDF extracts every ATCUD QR code from the PDF and returns each one
// fully decoded into structured, human-readable fields (seller NIF, buyer NIF,
// document type, tax breakdown, totals, etc.).
func (s *ScanService) ParsePDF(pdfBytes []byte) (*ParseResult, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("the PDF file is empty")
	}

	raw, err := s.extractFromPDF(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting QR codes from PDF: %w", err)
	}

	result := &ParseResult{
		TotalQRCodes: len(raw),
		Documents:    make([]domain.ParsedQRCode, 0),
	}

	seen := make(map[string]struct{})
	for _, r := range raw {
		atcud, hasATCUD := domain.DetectATCUD(r.Content)
		if !hasATCUD {
			continue
		}
		if _, dup := seen[atcud]; dup {
			continue
		}
		seen[atcud] = struct{}{}

		parsed, err := domain.ParseQRCode(r.Content)
		if err != nil {
			// Non-fatal: include a minimal entry so the caller knows something was found.
			result.Documents = append(result.Documents, domain.ParsedQRCode{
				NumeroPagina:  r.PageNumber,
				ConteudoBruto: r.Content,
			})
			continue
		}

		parsed.NumeroPagina = r.PageNumber
		result.Documents = append(result.Documents, *parsed)
	}

	result.ParsedCount = len(result.Documents)
	return result, nil
}
