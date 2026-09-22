package sendmail

import (
	"context"
	"time"

	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/infrastructure/excel"
)

type Request struct {
	Meta   Meta     `json:"meta"`
	Emails []string `json:"emails"`
}

type Meta struct {
	StoreID    string `json:"storeID"`
	StoreName  string `json:"storeName"`
	POSVersion string `json:"posVersion"`
	FromDate   string `json:"fromDate"`
	ToDate     string `json:"toDate"`
}

func (m Meta) report(title string) report.Meta {
	return report.Meta{
		Title: title, StoreID: m.StoreID, StoreName: m.StoreName,
		From: m.FromDate, To: m.ToDate,
	}
}

// LowInventory sends no period in the subject or note, unlike the other reports.
func LowInventory(ctx context.Context, s email.Sender, r Request, items []LowInventoryItem, now time.Time) error {
	const name = "Low Inventory Report"
	data, err := LowInventoryWorkbook(items, r.Meta.report(name), now)
	if err != nil {
		return err
	}
	note, err := body(r.Meta.StoreName, name, "", "")
	if err != nil {
		return err
	}
	return s.Send(ctx, email.Message{
		ToEmails:    r.Emails,
		Subject:     r.Meta.StoreName + " - " + name,
		Body:        note,
		FileName:    "Low-Inventory-Report.xlsx",
		FileContent: attachExcel(data),
	})
}

type EmployeeSale struct {
	EmployeeID   string  `json:"employeeID"`
	EmployeeCode string  `json:"employeeCode"`
	EmployeeName string  `json:"employeeName"`
	QuantitySold float64 `json:"quantitySold"`
	Discount     float64 `json:"discount"`
	NetSales     float64 `json:"netSales"`
	Tax          float64 `json:"tax"`
	NetSalesTax  float64 `json:"netSalesTax"`
}

type ItemSales struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	SKU             string         `json:"sku"`
	SaleByEmployees []EmployeeSale `json:"saleByEmployees"`
}

var employeeLabels = []report.Label{
	{Prop: "employeeName", Name: "Employee Name", Type: "string"},
	{Prop: "quantitySold", Name: "Qty Sold", Type: "number", Format: "#,##0"},
	{Prop: "discount", Name: "Discount", Type: "currency"},
	{Prop: "netSales", Name: "Net Sales", Type: "currency"},
	{Prop: "tax", Name: "Tax Amount", Type: "currency"},
	{Prop: "netSalesTax", Name: "Net Sales + Tax", Type: "currency"},
}

// ItemSalesByEmployeeWorkbook writes an overall summary, then one table per item ending in its own summary row.
func ItemSalesByEmployeeWorkbook(items []ItemSales, m report.Meta, now time.Time) ([]byte, error) {
	return build(m, now, func(g *excel.Grid) error {
		var all EmployeeSale
		for _, item := range items {
			for _, e := range item.SaleByEmployees {
				all = add(all, e)
			}
		}
		if err := g.Table(report.Table{
			// the overall summary has no name column
			Labels: employeeLabels[1:],
			Rows:   []report.Row{saleRow(all, "")},
		}, excel.TableOpts{Title: "Summary"}); err != nil {
			return err
		}
		g.BlankRow()
		g.BlankRow()

		for _, item := range items {
			var total EmployeeSale
			rows := make([]report.Row, 0, len(item.SaleByEmployees)+1)
			for _, e := range item.SaleByEmployees {
				total = add(total, e)
				rows = append(rows, saleRow(e, e.EmployeeName))
			}
			rows = append(rows, saleRow(total, "Summary"))
			if err := g.Table(report.Table{Labels: employeeLabels, Rows: rows}, excel.TableOpts{
				Title:    item.Name + " - SKU: " + item.SKU,
				TotalRow: report.TotalRow("employeeName"),
			}); err != nil {
				return err
			}
			g.BlankRow()
		}
		return nil
	})
}

func add(a, b EmployeeSale) EmployeeSale {
	a.QuantitySold += b.QuantitySold
	a.Discount += b.Discount
	a.NetSales += b.NetSales
	a.Tax += b.Tax
	a.NetSalesTax += b.NetSalesTax
	return a
}

func saleRow(e EmployeeSale, name string) report.Row {
	return report.Row{
		"employeeName": name, "quantitySold": e.QuantitySold, "discount": e.Discount,
		"netSales": e.NetSales, "tax": e.Tax, "netSalesTax": e.NetSalesTax,
	}
}

func ItemSalesByEmployee(ctx context.Context, s email.Sender, r Request, items []ItemSales, now time.Time) error {
	const name = "Item Sales by Employee Report"
	data, err := ItemSalesByEmployeeWorkbook(items, r.Meta.report(name), now)
	if err != nil {
		return err
	}
	note, err := body(r.Meta.StoreName, name, r.Meta.FromDate, r.Meta.ToDate)
	if err != nil {
		return err
	}
	return s.Send(ctx, excelMail(r, name, note, "Item-Sales-by-Employee-Report.xlsx", data))
}

type Modifier struct {
	ID         string  `json:"id"`
	Name       *string `json:"name"`
	Quantity   float64 `json:"quantity"`
	GrossSales float64 `json:"grossSales"`
}

type ModifierCategory struct {
	CategoryID        int        `json:"categoryID"`
	CategoryName      string     `json:"categoryName"`
	CategoryNameAlias string     `json:"categoryNameAlias"`
	Modifiers         []Modifier `json:"modifiers"`
	Summary           Modifier   `json:"summary"`
}

type ModifierData struct {
	Categories []ModifierCategory `json:"listSaleByCategoryID"`
}

// SalesByModifierWorkbook writes one table per category, each ending in a summary row.
func SalesByModifierWorkbook(d ModifierData, m report.Meta, now time.Time) ([]byte, error) {
	labels := []report.Label{
		{Prop: "name", Name: "Modifier", Type: "string"},
		{Prop: "quantity", Name: "Quantity", Type: "number", Format: "#,##0"},
		{Prop: "grossSales", Name: "Gross Sales", Type: "currency"},
	}
	return build(m, now, func(g *excel.Grid) error {
		for _, c := range d.Categories {
			rows := make([]report.Row, 0, len(c.Modifiers)+1)
			for _, mod := range c.Modifiers {
				rows = append(rows, modifierRow(mod, name(mod.Name)))
			}
			rows = append(rows, modifierRow(c.Summary, "Summary"))
			if err := g.Table(report.Table{Labels: labels, Rows: rows}, excel.TableOpts{
				Title:    c.CategoryNameAlias,
				TotalRow: report.TotalRow("name"),
			}); err != nil {
				return err
			}
			g.BlankRow()
		}
		return nil
	})
}

func modifierRow(mod Modifier, label any) report.Row {
	return report.Row{"name": label, "quantity": mod.Quantity, "grossSales": mod.GrossSales}
}

// Node wrote an empty cell for a null modifier name.
func name(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func SalesByModifier(ctx context.Context, s email.Sender, r Request, d ModifierData, now time.Time) error {
	const title = "Sales by Modifier Report"
	data, err := SalesByModifierWorkbook(d, r.Meta.report(title), now)
	if err != nil {
		return err
	}
	note, err := body(r.Meta.StoreName, title, r.Meta.FromDate, r.Meta.ToDate)
	if err != nil {
		return err
	}
	return s.Send(ctx, excelMail(r, title, note, "Sales-by-Modifier-Report"+fileStamp(r.Meta.FromDate, r.Meta.ToDate)+".xlsx", data))
}
