// Package excel writes .xlsx like the Node service: a cursor Sheet for the export, a row-appending Grid for mails.
package excel

import (
	"time"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/pkg/datetime"

	"github.com/xuri/excelize/v2"
)

// Number formats of ExcelUtilsHelper.insert.
const (
	fmtCurrency = "$#,##0.00"
	fmtPercent  = "0.00%"
	fmtDate     = "mm/dd/yyyy"
	bodyFont    = 12
	titleFont   = 20
	sectionFont = 14
)

// ExcelJS writes these as standard OOXML ids rather than custom formats.
var builtinNumFmt = map[string]int{
	"0":        1,
	"0.00":     2,
	"#,##0":    3,
	"#,##0.00": 4,
	"0%":       9,
	"0.00%":    10,
	"mm-dd-yy": 14,
	"@":        49,
}

type Book struct {
	f      *excelize.File
	styles map[style]int
	sheets int
}

// The first sheet renames excelize's default Sheet1 so no stray sheet remains.
func (b *Book) sheetNamed(name string) error {
	first := b.f.GetSheetName(0)
	b.sheets++
	switch {
	case name == first:
		return nil
	case b.sheets == 1:
		return b.f.SetSheetName(first, name)
	default:
		_, err := b.f.NewSheet(name)
		return err
	}
}

// style stays comparable: it keys the memo map.
type style struct {
	size   float64
	bold   bool
	italic bool
	// RRGGBB
	color        string
	numFmt       string
	alignH       string
	alignV       string
	wrap         bool
	underline    bool
	borderBottom bool
	borders      borderSet
}

type borderSet struct{ top, right, bottom, left borderSide }

// An empty style means no line.
type borderSide struct {
	// "thin" or "double"
	style string
	color string
}

var borderStyleID = map[string]int{"thin": 1, "double": 6}

// New starts a workbook with the document properties of the Node export.
func New(now time.Time) (*Book, error) {
	f := excelize.NewFile()
	err := f.SetDocProps(&excelize.DocProperties{
		Creator:        "dev@blogicsystems.com",
		LastModifiedBy: "dev@blogicsystems.com",
		Title:          "BLogic Systems Export",
		Created:        now.UTC().Format(time.RFC3339),
	})
	return &Book{f: f, styles: map[style]int{}}, err
}

// Sheet returns a cursor writer, creating the sheet unless it is excelize's default.
func (b *Book) Sheet(name string) (*Sheet, error) {
	if err := b.sheetNamed(name); err != nil {
		return nil, err
	}
	return &Sheet{b: b, name: name, Cursor: 1}, nil
}

func (b *Book) Bytes() ([]byte, error) {
	buf, err := b.f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), b.f.Close()
}

// excelize deduplicates styles itself; the memo only skips rebuilding the struct.
func (b *Book) styleID(s style) (int, error) {
	if id, ok := b.styles[s]; ok {
		return id, nil
	}
	st := &excelize.Style{}
	// No Font when nothing is set: excelize then uses the workbook default, Calibri 11, like ExcelJS.
	if s.size != 0 || s.bold || s.italic || s.color != "" || s.underline {
		st.Font = &excelize.Font{Size: s.size, Bold: s.bold, Italic: s.italic, Color: s.color}
		if s.underline {
			st.Font.Underline = "single"
		}
	}
	if id, ok := builtinNumFmt[s.numFmt]; ok {
		st.NumFmt = id
	} else if s.numFmt != "" {
		st.CustomNumFmt = &s.numFmt
	}
	if s.alignH != "" || s.alignV != "" || s.wrap {
		st.Alignment = &excelize.Alignment{Horizontal: s.alignH, Vertical: s.alignV, WrapText: s.wrap}
	}
	if s.borderBottom {
		st.Border = []excelize.Border{{Type: "bottom", Style: 1}}
	}
	for _, side := range []struct {
		name string
		edge borderSide
	}{
		{"top", s.borders.top}, {"right", s.borders.right},
		{"bottom", s.borders.bottom}, {"left", s.borders.left},
	} {
		if side.edge.style == "" {
			continue
		}
		st.Border = append(st.Border, excelize.Border{
			Type: side.name, Style: borderStyleID[side.edge.style], Color: side.edge.color,
		})
	}
	id, err := b.f.NewStyle(st)
	if err != nil {
		return 0, err
	}
	b.styles[s] = id
	return id, nil
}

type Sheet struct {
	b    *Book
	name string
	// Cursor is the next 1-based row.
	Cursor int
}

// nil leaves the cell empty but styled, as Node's undefined did.
func (b *Book) set(sheet string, row, col int, v any, st style) error {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	id, err := b.styleID(st)
	if err != nil {
		return err
	}
	if err := b.f.SetCellStyle(sheet, cell, cell, id); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return b.f.SetCellValue(sheet, cell, v)
}

func (s *Sheet) set(row, col int, v any, st style) error { return s.b.set(s.name, row, col, v, st) }

// Meta writes the title/generated-at and period/store rows and leaves the cursor below them.
func (s *Sheet) Meta(m report.Meta, now time.Time) error {
	if m.Title != "" {
		if err := s.set(s.Cursor, 1, m.Title, style{size: titleFont, bold: true}); err != nil {
			return err
		}
	}
	if m.From != "" {
		// The stamp is rendered in the report's own zone, like the Node version.
		if from, ok := datetime.ParseISO(m.From); ok {
			now = now.In(from.Location())
		}
		if err := s.set(s.Cursor, 8, "Generated on: "+now.Format("01/02/2006 03:04 PM"), style{size: bodyFont}); err != nil {
			return err
		}
	}
	s.Cursor++
	if m.From != "" {
		if err := s.set(s.Cursor, 1, datetime.FormatDates(m.From, m.To), style{size: bodyFont}); err != nil {
			return err
		}
	}
	if m.StoreName != "" {
		if err := s.set(s.Cursor, 8, m.StoreName+" ("+m.StoreID+")", style{size: bodyFont}); err != nil {
			return err
		}
	}
	s.Cursor++
	return nil
}

// DayPartTable writes a weekday block as Node's addTable did: title and headers share a row, column A repeats the day part.
func (s *Sheet) DayPartTable(t report.Table) error {
	s.Cursor++
	if t.Title != "" {
		if err := s.set(s.Cursor, 1, t.Title, style{size: sectionFont, bold: true}); err != nil {
			return err
		}
	}
	if err := s.header(s.Cursor, t.Labels, 2); err != nil {
		return err
	}
	for i, row := range t.Rows {
		at := s.Cursor + i + 1
		if err := s.set(at, 1, row["dayPart"], style{size: bodyFont}); err != nil {
			return err
		}
		if err := s.insert(at, t.Labels, row, 2); err != nil {
			return err
		}
	}
	s.Cursor += len(t.Rows)
	return nil
}

func (s *Sheet) header(row int, labels []report.Label, startCol int) error {
	for i, l := range labels {
		name := l.Name
		if name == "" {
			name = l.Prop
		}
		st := style{size: bodyFont, bold: true, borderBottom: true}
		if isNumeric(l.Type) {
			st.alignH = "right"
		}
		if err := s.set(row, startCol+i, name, st); err != nil {
			return err
		}
		if l.Note == "" {
			continue
		}
		cell, err := excelize.CoordinatesToCellName(startCol+i, row)
		if err != nil {
			return err
		}
		if err := s.b.f.AddComment(s.name, excelize.Comment{
			Cell:      cell,
			Author:    "BLogic Systems",
			Paragraph: []excelize.RichTextRun{{Text: l.Note}},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sheet) insert(row int, labels []report.Label, r report.Row, startCol int) error {
	for i, l := range labels {
		v := r[l.Prop]
		st := style{size: bodyFont, alignV: "top"}
		switch l.Type {
		case "currency":
			st.numFmt = fmtCurrency
		case "number":
			// Node only sets a format here when the label carries one.
			if l.Format != "" {
				st.numFmt = l.Format
			}
		case "percent":
			st.numFmt = or(l.Format, fmtPercent)
		case "date":
			st.numFmt = or(l.Format, fmtDate)
			// excelize writes the value's own wall clock, which is what Node's local-as-UTC re-parse produced.
			if iso, ok := v.(string); ok {
				if t, ok := datetime.ParseISO(iso); ok {
					v = t
				}
			}
		}
		// Numbers arriving as text are right aligned, as in Node.
		if _, isText := v.(string); isText && isNumeric(l.Type) {
			st.alignH = "right"
		}
		if err := s.set(row, startCol+i, v, st); err != nil {
			return err
		}
	}
	return nil
}

// SetWidths applies fixed column widths from column A rightwards.
func (s *Sheet) SetWidths(widths []float64) error {
	for i, w := range widths {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := s.b.f.SetColWidth(s.name, col, col, w); err != nil {
			return err
		}
	}
	return nil
}

func isNumeric(t string) bool {
	switch t {
	case "currency", "number", "date", "percent":
		return true
	}
	return false
}

func or(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
