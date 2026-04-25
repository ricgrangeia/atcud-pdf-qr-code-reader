package pdf

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"
	"os/exec"
	"strings"

	"github.com/makiuchi-d/gozxing"
	qrmulti "github.com/makiuchi-d/gozxing/multi/qrcode"
)

// RawQRCode is the output of the scanner: the decoded text and which page it came from.
// The application layer turns this into a domain QRCode after applying business rules.
type RawQRCode struct {
	Content    string
	PageNumber int
}

// ExtractQRCodes scans every page of a PDF and returns all decoded QR codes.
//
// Flow:
//  1. Write the PDF bytes to a temp file.
//  2. Render each page to a PNG image with pdftoppm.
//  3. Scan each PNG for QR codes using the ZXing library (gozxing).
//  4. Return all decoded results, preserving the page number.
func ExtractQRCodes(pdfBytes []byte) ([]RawQRCode, error) {
	// Write PDF to disk so pdftoppm can process it.
	tmpFile, err := os.CreateTemp("", "input_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("creating temp PDF file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err = tmpFile.Write(pdfBytes); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing PDF to temp file: %w", err)
	}
	tmpFile.Close()

	// Reject decompression-bomb PDFs before paying the rendering cost.
	if n := pageCount(tmpFile.Name()); n > MaxPDFPages {
		return nil, fmt.Errorf("PDF has %d pages — limit is %d", n, MaxPDFPages)
	}

	// Render all PDF pages to PNG images.
	pageFiles, cleanup, err := renderPagesToImages(tmpFile.Name())
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var results []RawQRCode

	for pageIdx, imagePath := range pageFiles {
		pageNumber := pageIdx + 1

		codes, scanErr := scanImageForQRCodes(imagePath)
		if scanErr != nil {
			// A single bad page should not abort the whole document.
			// The caller will see fewer results, not an error.
			continue
		}

		for _, code := range codes {
			results = append(results, RawQRCode{
				Content:    code,
				PageNumber: pageNumber,
			})
		}
	}

	// Text fallback: some PDFs (e.g. Via Verde) embed ATCUD as plain text alongside
	// QR code images that ZXing cannot decode. pdftotext (poppler-utils) extracts the
	// text layer and reconstructs AT-format strings where possible.
	textResults := extractATCUDsFromText(tmpFile.Name())
	results = append(results, textResults...)

	return results, nil
}

// ExtractQRCodesFromImage scans a single image (JPEG, PNG, GIF, …) for QR codes
// and returns all decoded results. PageNumber is always 1.
func ExtractQRCodesFromImage(imageBytes []byte) ([]RawQRCode, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("image is empty")
	}

	texts, err := decodeQRCodesFromBytes(imageBytes)
	if err != nil {
		return nil, err
	}

	results := make([]RawQRCode, 0, len(texts))
	for _, t := range texts {
		results = append(results, RawQRCode{Content: t, PageNumber: 1})
	}
	return results, nil
}

// scanImageForQRCodes finds every QR code in one image file.
// It tries gozxing first; if nothing is found it falls back to zbarimg.
func scanImageForQRCodes(imagePath string) ([]string, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("opening image %s: %w", imagePath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding image %s: %w", imagePath, err)
	}

	texts, err := scanBitmap(img)
	if err != nil || len(texts) > 0 {
		return texts, err
	}

	// gozxing found nothing — try zbarimg (handles more QR code variants).
	return zbarimgScan(imagePath)
}

// decodeQRCodesFromBytes decodes QR codes directly from raw image bytes without a temp file.
func decodeQRCodesFromBytes(data []byte) ([]string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	return scanBitmap(img)
}

// scanBitmap converts an image to a ZXing bitmap and extracts all QR codes from it.
func scanBitmap(img image.Image) ([]string, error) {
	// Convert the image to the binary bitmap format that ZXing expects.
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("creating bitmap from image: %w", err)
	}

	// TRY_HARDER makes ZXing attempt more aggressive scanning strategies,
	// which improves detection for small or slightly skewed QR codes.
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}

	// QRCodeMultiReader finds all QR codes in one image (not just the first one).
	reader := qrmulti.NewQRCodeMultiReader()
	decoded, err := reader.DecodeMultiple(bitmap, hints)
	if err != nil {
		// "not found" is not an error — the page just has no QR codes.
		return nil, nil
	}

	texts := make([]string, 0, len(decoded))
	for _, result := range decoded {
		texts = append(texts, result.GetText())
	}
	return texts, nil
}

// zbarimgScan uses the zbarimg CLI (zbar-tools) to decode QR codes from an image file.
// This is a fallback for QR codes that gozxing cannot decode.
func zbarimgScan(imagePath string) ([]string, error) {
	out, err := exec.Command("zbarimg", "--raw", "-q", imagePath).Output()
	if err != nil {
		// exit code 4 means no barcodes found — not an error.
		return nil, nil
	}
	var results []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			results = append(results, line)
		}
	}
	return results, nil
}
