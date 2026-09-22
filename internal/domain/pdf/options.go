package pdf

import (
	"strconv"
	"strings"
)

// Request is the JSON body of an export-pdf call: ready-made HTML or the tables to render.
type Request struct {
	HTMLContent string `json:"htmlContent"`
	FileName    string `json:"fileName"`
	Format      string `json:"format"`
	// Width and Height: number or CSS length ("8.5in", "1240px").
	Width     any       `json:"width"`
	Height    any       `json:"height"`
	Landscape bool      `json:"landscape"`
	Margin    *Margin   `json:"margin"`
	Datasets  *Datasets `json:"datasets"`
}

// Margin holds the four CSS lengths; empty strings mean zero, as in Node.
type Margin struct {
	Top    any `json:"top"`
	Right  any `json:"right"`
	Bottom any `json:"bottom"`
	Left   any `json:"left"`
}

// Print is the normalised instruction for the renderer, in inches as printToPDF expects.
type Print struct {
	PaperWidth  float64
	PaperHeight float64
	MarginTop   float64
	MarginRight float64
	MarginBot   float64
	MarginLeft  float64
	Landscape   bool
}

// paperSizes are the puppeteer page formats, in inches.
var paperSizes = map[string][2]float64{
	"letter":  {8.5, 11},
	"legal":   {8.5, 14},
	"tabloid": {11, 17},
	"ledger":  {17, 11},
	"a0":      {33.1, 46.8},
	"a1":      {23.4, 33.1},
	"a2":      {16.54, 23.4},
	"a3":      {11.7, 16.54},
	"a4":      {8.27, 11.7},
	"a5":      {5.83, 8.27},
	"a6":      {4.13, 5.83},
}

// Normalize resolves the page in inches: a named format wins over width/height, as in puppeteer; Letter is the fallback.
func (r Request) Normalize() Print {
	p := Print{PaperWidth: 8.5, PaperHeight: 11, Landscape: r.Landscape}
	if size, ok := paperSizes[strings.ToLower(r.Format)]; ok {
		p.PaperWidth, p.PaperHeight = size[0], size[1]
	} else if w, okW := inches(r.Width); okW {
		if h, okH := inches(r.Height); okH {
			p.PaperWidth, p.PaperHeight = w, h
		}
	}
	if r.Margin != nil {
		p.MarginTop, _ = inches(r.Margin.Top)
		p.MarginRight, _ = inches(r.Margin.Right)
		p.MarginBot, _ = inches(r.Margin.Bottom)
		p.MarginLeft, _ = inches(r.Margin.Left)
	}
	return p
}

// A bare number is pixels, as in puppeteer.
func inches(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n / 96, true
	case int:
		return float64(n) / 96, true
	case string:
		s := strings.TrimSpace(strings.ToLower(n))
		if s == "" {
			return 0, false
		}
		unit, div := "px", 96.0
		for u, d := range map[string]float64{"px": 96, "in": 1, "cm": 2.54, "mm": 25.4} {
			if strings.HasSuffix(s, u) {
				unit, div = u, d
				break
			}
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, unit)), 64)
		if err != nil {
			return 0, false
		}
		return f / div, true
	}
	return 0, false
}

// SanitizeFileName keeps letters, digits and spaces and drops a trailing .pdf, byte-identical to Node.
func SanitizeFileName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSuffix(name, ".pdf") {
		if r == ' ' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
