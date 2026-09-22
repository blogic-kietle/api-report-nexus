// Package exportpdf turns an export-pdf request into PDF bytes.
package exportpdf

import (
	"context"
	"errors"

	"api-report-nexus/internal/domain/pdf"
	"api-report-nexus/internal/pkg/compress"
)

var errNoRows = errors.New("exportpdf: payload contains no rows")

// Renderer is the PDF service this use case prints with; Merge joins the pages of a split render.
type Renderer interface {
	Render(ctx context.Context, html string, layout pdf.Print) ([]byte, error)
	Merge(ctx context.Context, parts [][]byte) ([]byte, error)
}

// HTML renders a request that already carries markup, gunzipping it first when the client sent gzip+base64.
func HTML(ctx context.Context, r Renderer, req pdf.Request) (name string, data []byte, err error) {
	html := req.HTMLContent
	if compress.IsBase64(html) {
		if html, err = compress.DecodeGzipBase64(req.HTMLContent); err != nil {
			return "", nil, err
		}
	}
	data, err = r.Render(ctx, html, req.Normalize())
	if err != nil {
		return "", nil, err
	}
	return pdf.SanitizeFileName(req.FileName), data, nil
}
