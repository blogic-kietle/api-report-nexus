package exportpdf

import (
	"context"
	"html/template"
	"strings"

	"api-report-nexus/assets"
	"api-report-nexus/internal/domain/pdf"
	"api-report-nexus/internal/pkg/datetime"
)

var generalTable = template.Must(template.ParseFS(assets.FS, "templates/general-table.html"))

// renders is how many chunks are printed at once; Gotenberg runs 6 Chromium conversions concurrently by default.
const renders = 6

// Datasets prints payloads of chunkSize rows or more in pieces and merges them: Chromium refuses pages that are too large.
func Datasets(ctx context.Context, r Renderer, req pdf.Request, chunkSize int) (string, []byte, error) {
	name := pdf.SanitizeFileName(req.FileName)
	d := req.Datasets
	if d.Meta != nil && len(d.Meta.Dates) == 2 {
		d.Meta.Date = datetime.FormatDates(d.Meta.Dates[0], d.Meta.Dates[1])
	}
	layout := req.Normalize()

	if d.Tables.RowCount() < chunkSize {
		html, err := render(pdf.View{ShowHeader: true, Store: d.Meta, Title: req.FileName, Tables: d.Tables})
		if err != nil {
			return "", nil, err
		}
		data, err := r.Render(ctx, html, layout)
		if err != nil {
			return "", nil, err
		}
		return name, data, nil
	}

	var views []pdf.View
	for _, table := range d.Tables {
		for start := 0; start < len(table.Rows); start += chunkSize {
			piece := table
			piece.Rows = table.Rows[start:min(start+chunkSize, len(table.Rows))]
			view := pdf.View{Title: req.FileName, Tables: pdf.Tables{piece}}
			// Only the very first page carries the store letterhead.
			if len(views) == 0 {
				view.ShowHeader, view.Store = d.Meta != nil, d.Meta
			}
			views = append(views, view)
		}
	}
	if len(views) == 0 {
		return "", nil, errNoRows
	}

	// The first failure cancels the renders still in flight.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	parts := make([][]byte, len(views))
	errs := make(chan error, len(views))
	slots := make(chan struct{}, renders)
	for i, v := range views {
		slots <- struct{}{}
		go func() {
			defer func() { <-slots }()
			html, err := render(v)
			if err == nil {
				parts[i], err = r.Render(ctx, html, layout)
			}
			errs <- err
		}()
	}
	for range views {
		if err := <-errs; err != nil {
			return "", nil, err
		}
	}
	merged, err := r.Merge(ctx, parts)
	if err != nil {
		return "", nil, err
	}
	return name, merged, nil
}

func render(v pdf.View) (string, error) {
	var buf strings.Builder
	if err := generalTable.Execute(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}
