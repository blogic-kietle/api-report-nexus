package excel

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/pkg/datetime"
)

// Font sizes and borders of ExcelTableGenerator.
const (
	gridTitleFont    = 16
	gridSubTitleFont = 14
	gridNoteFont     = 12
	gridHeaderFont   = 12
	gridContentFont  = 11
	maxColumnWidth   = 17
	gridDateFmt      = "mm/dd/yyyy hh:mm AM/PM"
	// ExcelJS argb, written through unchanged
	linkColour = "1F497DFF"
)

var (
	grayEdge   = borderSide{style: "thin", color: "BFBFBF"}
	blackEdge  = borderSide{style: "thin", color: "000000"}
	doubleEdge = borderSide{style: "double"}

	headerBorders  = borderSet{top: grayEdge, right: grayEdge, bottom: blackEdge, left: grayEdge}
	contentBorders = borderSet{top: grayEdge, right: grayEdge, bottom: grayEdge, left: grayEdge}
)

// Grid appends rows in order; it backs the workbooks the send-email reports attach.
type Grid struct {
	b    *Book
	name string
	// last row written, 1-based
	row int
	// longest rendered value per column, for auto width
	widest map[int]int
}

// Grid creates a sheet laid out for printing: A4 landscape, fit to one page wide.
func (b *Book) Grid(name string) (*Grid, error) {
	if err := b.sheetNamed(name); err != nil {
		return nil, err
	}
	g := &Grid{b: b, name: name, widest: map[int]int{}}
	landscape, paper, fitWidth, fitHeight := "landscape", 9, 1, 0
	if err := b.f.SetPageLayout(name, &excelize.PageLayoutOptions{
		Orientation: &landscape, Size: &paper, FitToWidth: &fitWidth, FitToHeight: &fitHeight,
	}); err != nil {
		return nil, err
	}
	fit := true
	if err := b.f.SetSheetProps(name, &excelize.SheetPropsOptions{FitToPage: &fit}); err != nil {
		return nil, err
	}
	margin := 0.25
	zero := 0.0
	return g, b.f.SetPageMargins(name, &excelize.PageLayoutMarginsOptions{
		Top: &margin, Left: &margin, Bottom: &margin, Right: &margin, Header: &zero, Footer: &zero,
	})
}

func (g *Grid) set(row, col int, v any, st style) error {
	if n := len(jsString(v)); n > g.widest[col] {
		g.widest[col] = n
	}
	return g.b.set(g.name, row, col, v, st)
}

// Meta writes the header rows: title and timestamp, period and store, notes, two blank rows.
func (g *Grid) Meta(m report.Meta, notes []string, now time.Time) error {
	if m.Title != "" {
		g.row++
		if err := g.set(g.row, 1, m.Title, style{size: gridTitleFont, bold: true}); err != nil {
			return err
		}
	}
	if from, ok := datetime.ParseISO(m.From); ok {
		now = now.In(from.Location())
	}
	// Column N, and a single-digit hour unlike the cursor writer's two digits.
	if err := g.set(1, 14, "Generated on: "+now.Format("01/02/2006 3:04 PM"), style{}); err != nil {
		return err
	}
	if m.From != "" || m.To != "" {
		g.row++
		if err := g.set(g.row, 1, datetime.FormatDatesFull(m.From, m.To), style{}); err != nil {
			return err
		}
	}
	if m.StoreName != "" {
		if err := g.set(2, 14, m.StoreName+" ("+m.StoreID+")", style{}); err != nil {
			return err
		}
	}
	if len(notes) > 0 {
		g.row++
		for _, note := range notes {
			g.row++
			if err := g.set(g.row, 1, note, style{
				size: gridNoteFont, italic: true, color: "214567", alignH: "left",
			}); err != nil {
				return err
			}
		}
	}
	g.row += 2
	return nil
}

type TableOpts struct {
	Title string
	// StartCol defaults to 1.
	StartCol int
	// TotalRow marks rows to bold and cap with a double rule.
	TotalRow func(report.Row) bool
	// Bold marks rows to bold only, e.g. group headings.
	Bold func(report.Row) bool
}

func (g *Grid) BlankRow() { g.row++ }

// Table writes header, rows and a totals row for CalculateTotal columns; a horizontal table puts each label on its own row.
func (g *Grid) Table(t report.Table, o TableOpts) error {
	if len(t.Rows) == 0 {
		// Node skips empty tables entirely
		return nil
	}
	start := max(o.StartCol, 1)

	if o.Title != "" {
		g.row++
		if err := g.set(g.row, start, o.Title, style{size: gridSubTitleFont, bold: true}); err != nil {
			return err
		}
	}

	if t.Horizontal() {
		if err := g.horizontal(t, start); err != nil {
			return err
		}
		return g.autoWidth(t.Labels, start)
	}

	g.row++
	for i, l := range t.Labels {
		st := style{
			size: gridHeaderFont, bold: true, wrap: true,
			alignH: alignFor(l.Type), alignV: "top", borders: headerBorders,
		}
		if err := g.set(g.row, start+i, l.Name, st); err != nil {
			return err
		}
		if l.Note != "" {
			if err := g.note(g.row, start+i, l.Note); err != nil {
				return err
			}
		}
	}

	totals := map[string]float64{}
	for _, row := range t.Rows {
		g.row++
		total := o.TotalRow != nil && o.TotalRow(row)
		bold := total || (o.Bold != nil && o.Bold(row))
		for i, l := range t.Labels {
			st := style{
				size: gridContentFont, bold: l.Bold || bold, wrap: true,
				alignH: alignFor(l.Type), borders: contentBorders,
			}
			if total {
				st.borders.top = doubleEdge
			}
			value, numFmt := cellValue(l, row[l.Prop])
			st.numFmt = numFmt
			if l.Type == "link" {
				if link, ok := row["link"].(string); ok && link != "" {
					if err := g.link(g.row, start+i, value, link, st); err != nil {
						return err
					}
					continue
				}
			}
			if err := g.set(g.row, start+i, value, st); err != nil {
				return err
			}
			// Note "int" is absent: Node only totals these three types.
			if l.CalculateTotal && (l.Type == "number" || l.Type == "currency" || l.Type == "percent") {
				totals[l.Prop] += toFloat(row[l.Prop])
			}
		}
	}

	if len(totals) > 0 {
		g.row++
		for i, l := range t.Labels {
			// Untotalled columns are only bolded and capped: Node gave them no alignment or wrapping.
			st := style{size: gridContentFont, bold: true, borders: borderSet{top: doubleEdge}}
			var value any
			if total, ok := totals[l.Prop]; ok {
				value, st.numFmt = cellValue(l, total)
				st.alignH, st.wrap = alignFor(l.Type), true
			} else if i == 0 {
				value = "Total"
			}
			if err := g.set(g.row, start+i, value, st); err != nil {
				return err
			}
		}
	}

	return g.autoWidth(t.Labels, start)
}

func (g *Grid) horizontal(t report.Table, start int) error {
	record := t.Rows[0]
	for _, l := range t.Labels {
		g.row++
		if err := g.set(g.row, start, l.Name, style{
			size: gridHeaderFont, bold: l.Bold, borders: contentBorders,
		}); err != nil {
			return err
		}
		value, numFmt := cellValue(l, record[l.Prop])
		if err := g.set(g.row, start+1, value, style{
			size: gridContentFont, bold: l.Bold, wrap: true, numFmt: numFmt,
			alignH: alignFor(l.Type), borders: contentBorders,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (g *Grid) link(row, col int, text any, url string, st style) error {
	st.color, st.underline = linkColour, true
	if err := g.set(row, col, text, st); err != nil {
		return err
	}
	// Node measured the width of a link cell as String({text,hyperlink}).
	g.widest[col] = max(g.widest[col], len("[object Object]"))
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	return g.b.f.SetCellHyperLink(g.name, cell, url, "External")
}

func (g *Grid) note(row, col int, text string) error {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	return g.b.f.AddComment(g.name, excelize.Comment{
		Cell: cell, Author: "BLogic Systems",
		Paragraph: []excelize.RichTextRun{{Text: text}},
	})
}

// Later tables overwrite earlier widths: Node recomputed after every table, an assignment not a max.
func (g *Grid) autoWidth(labels []report.Label, start int) error {
	for i, l := range labels {
		col := start + i
		width := math.Min(float64(max(len(l.Name), max(g.widest[col], 10))+5), maxColumnWidth)
		if l.Width > 0 {
			// an explicit width wins and is not capped
			width = float64(l.Width)
		}
		name, err := excelize.ColumnNumberToName(col)
		if err != nil {
			return err
		}
		if err := g.b.f.SetColWidth(g.name, name, name, width); err != nil {
			return err
		}
	}
	return nil
}

func cellValue(l report.Label, v any) (any, string) {
	switch l.Type {
	case "currency":
		if s, ok := v.(string); ok && s == "empty" {
			// sentinel for a deliberately blank cell
			return "", or(l.Format, fmtCurrency)
		}
		return toFloat(v), or(l.Format, fmtCurrency)
	case "number":
		return toFloat(v), or(l.Format, "#,##0.00")
	case "percent":
		return toFloat(v), or(l.Format, "#,##0.00%")
	case "int":
		return toFloat(v), or(l.Format, "#,##0")
	case "date":
		if iso, ok := v.(string); ok {
			if t, ok := datetime.ParseISO(iso); ok {
				return t, or(l.Format, gridDateFmt)
			}
		}
		return v, or(l.Format, gridDateFmt)
	}
	if v == nil {
		return "", ""
	}
	return v, ""
}

func alignFor(t string) string {
	if isNumericType(t) {
		return "right"
	}
	return "left"
}

func isNumericType(t string) bool {
	switch t {
	case "number", "currency", "percent", "date", "int":
		return true
	}
	return false
}

// toFloat is JavaScript's `Number(value) || 0`.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

// jsString is `String(cell.value || ”)`, what Node measured widths with: falsy counts as empty.
func jsString(v any) string {
	switch n := v.(type) {
	case nil:
		return ""
	case string:
		return n
	case bool:
		if !n {
			return ""
		}
		return "true"
	case float64:
		if n == 0 {
			return ""
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	case time.Time:
		return n.String()
	}
	return fmt.Sprint(v)
}
