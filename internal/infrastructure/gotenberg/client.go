package gotenberg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"api-report-nexus/internal/domain/pdf"
)

// ErrBusy is Gotenberg's 503: queue full, its own timeout, or a Chromium crash. Callers may retry.
var ErrBusy = errors.New("gotenberg: busy")

// Node's printToPDF scale, kept: content wider than A4 landscape (841.89pt) zooms to fit; Blink alone stops at 1.5x and clips.
const fit = `<script>(()=>{const w=document.body.scrollWidth;if(w>841.89)document.documentElement.style.zoom=Math.max(.1,Math.round(84189/w)/100)})()</script>`

type Client struct{ base string }

func New(base string) *Client { return &Client{strings.TrimSuffix(base, "/")} }

func (c *Client) Render(ctx context.Context, html string, p pdf.Print) ([]byte, error) {
	in := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return c.post(ctx, "/forms/chromium/convert/html", map[string]string{
		"paperWidth": in(p.PaperWidth), "paperHeight": in(p.PaperHeight),
		"marginTop": in(p.MarginTop), "marginRight": in(p.MarginRight),
		"marginBottom": in(p.MarginBot), "marginLeft": in(p.MarginLeft),
		"landscape":       strconv.FormatBool(p.Landscape),
		"printBackground": "true", "emulatedMediaType": "screen",
	}, map[string][]byte{"index.html": []byte(html + fit)})
}

// Merge concatenates PDFs in order.
func (c *Client) Merge(ctx context.Context, parts [][]byte) ([]byte, error) {
	if len(parts) == 1 {
		return parts[0], nil
	}
	files := make(map[string][]byte, len(parts))
	for i, p := range parts {
		// Gotenberg merges in alphanumeric filename order.
		files[fmt.Sprintf("%04d.pdf", i)] = p
	}
	return c.post(ctx, "/forms/pdfengines/merge", nil, files)
}

func (c *Client) post(ctx context.Context, route string, fields map[string]string, files map[string][]byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	// Writes into a bytes.Buffer cannot fail.
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for name, data := range files {
		part, _ := w.CreateFormFile("files", name)
		_, _ = part.Write(data)
	}
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+route, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if resp.StatusCode == http.StatusServiceUnavailable {
			return nil, fmt.Errorf("%w: %s", ErrBusy, bytes.TrimSpace(detail))
		}
		return nil, fmt.Errorf("gotenberg: %s: %s", resp.Status, bytes.TrimSpace(detail))
	}
	return io.ReadAll(resp.Body)
}
