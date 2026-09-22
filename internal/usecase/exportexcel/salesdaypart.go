// Package exportexcel turns mapped report tables into .xlsx downloads.
package exportexcel

import (
	"time"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/infrastructure/excel"
	"api-report-nexus/internal/pkg/datetime"
)

// Column widths of the Sales by Day Part sheet, copied from the Node export.
var dayPartWidths = []float64{35, 25, 25, 25, 25, 25, 18, 18, 18}

// SalesDayPart renders one sheet with a table per weekday and returns the download name with the bytes.
func SalesDayPart(tables []report.Table, m report.Meta, now time.Time) (string, []byte, error) {
	book, err := excel.New(now)
	if err != nil {
		return "", nil, err
	}
	sheet, err := book.Sheet("Sheet1")
	if err != nil {
		return "", nil, err
	}
	if err := sheet.Meta(m, now); err != nil {
		return "", nil, err
	}
	sheet.Cursor++
	for _, t := range tables {
		if err := sheet.DayPartTable(t); err != nil {
			return "", nil, err
		}
		sheet.Cursor++
	}
	if err := sheet.SetWidths(dayPartWidths); err != nil {
		return "", nil, err
	}
	data, err := book.Bytes()
	if err != nil {
		return "", nil, err
	}
	return datetime.GenerateFileName(m.Title, m.From, m.To), data, nil
}
