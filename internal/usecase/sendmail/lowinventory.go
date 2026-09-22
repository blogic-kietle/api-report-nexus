// Package sendmail builds the report attachments and hands them to the License API.
package sendmail

import (
	"time"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/infrastructure/excel"
)

type LowInventoryItem struct {
	SKU               string  `json:"sku"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	CategoryName      string  `json:"categoryName"`
	CategoryNameAlias string  `json:"categoryNameAlias"`
	OnhandQuantity    float64 `json:"onhandQuantity"`
	ReorderPoint      float64 `json:"reorderPoint"`
	Cost              float64 `json:"cost"`
}

// LowInventoryWorkbook renders every item plus a totals line.
func LowInventoryWorkbook(items []LowInventoryItem, m report.Meta, now time.Time) ([]byte, error) {
	rows := make([]report.Row, 0, len(items)+1)
	var qty, reorder, cost float64
	for _, it := range items {
		qty += it.OnhandQuantity
		reorder += it.ReorderPoint
		cost += it.Cost
		rows = append(rows, report.Row{
			"sku": it.SKU, "description": it.Description, "categoryNameAlias": it.CategoryNameAlias,
			"onhandQuantity": it.OnhandQuantity, "reorderPoint": it.ReorderPoint, "cost": it.Cost,
		})
	}
	rows = append(rows, report.Row{
		"sku": "Total", "name": "", "isTotal": true, "description": "", "categoryNameAlias": "",
		"onhandQuantity": qty, "reorderPoint": reorder, "cost": cost,
	})

	table := report.Table{
		Labels: []report.Label{
			{Prop: "sku", Name: "SKU", Type: "string"},
			{Prop: "description", Name: "Description", Type: "string"},
			{Prop: "categoryNameAlias", Name: "Category", Type: "string"},
			{Prop: "onhandQuantity", Name: "On-hand Quantity", Type: "number", Format: "#,##0"},
			{Prop: "reorderPoint", Name: "Reorder Point", Type: "number", Format: "#,##0"},
			{Prop: "cost", Name: "Cost", Type: "currency"},
		},
		Rows: rows,
	}
	return build(m, now, func(g *excel.Grid) error {
		return g.Table(table, excel.TableOpts{TotalRow: report.TotalRow("name")})
	})
}

func build(m report.Meta, now time.Time, write func(*excel.Grid) error) ([]byte, error) {
	book, err := excel.New(now)
	if err != nil {
		return nil, err
	}
	grid, err := book.Grid("Sheet1")
	if err != nil {
		return nil, err
	}
	if err := grid.Meta(m, nil, now); err != nil {
		return nil, err
	}
	if err := write(grid); err != nil {
		return nil, err
	}
	return book.Bytes()
}
