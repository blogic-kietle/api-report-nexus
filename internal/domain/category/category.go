// Package category maps POS sales by category into the three Sales By Category tables
package category

import (
	"maps"
	"math"

	"api-report-nexus/internal/domain/report"
)

type Modifier struct {
	Description *string  `json:"description"`
	Quantity    float64  `json:"quantity"`
	GrossSales  float64  `json:"grossSales"`
	AvgPrice    *float64 `json:"avgPrice"`
}

type Item struct {
	Description        *string    `json:"description"`
	Quantity           float64    `json:"quantity"`
	TaxAmount          float64    `json:"taxAmount"`
	NetSales           float64    `json:"netSales"`
	GrossSales         float64    `json:"grossSales"`
	DiscountAmount     float64    `json:"discountAmount"`
	CashDiscountAmount float64    `json:"cashDiscountAmount"`
	CoinDiscountAmount float64    `json:"coinDiscountAmount"`
	MerchantFee        float64    `json:"merchantFee"`
	Cost               float64    `json:"cost"`
	Profit             float64    `json:"profit"`
	Modifiers          []Modifier `json:"modifiers"`
}

type Category struct {
	DepartmentNameAlias string  `json:"departmentNameAlias"`
	Quantity            float64 `json:"quantity"`
	TaxAmount           float64 `json:"taxAmount"`
	DiscountAmount      float64 `json:"discountAmount"`
	CashDiscountAmount  float64 `json:"cashDiscountAmount"`
	CoinDiscountAmount  float64 `json:"coinDiscountAmount"`
	NetSales            float64 `json:"netSales"`
	GrossSales          float64 `json:"grossSales"`
	MerchantFee         float64 `json:"merchantFee"`
	Cost                float64 `json:"cost"`
	Profit              float64 `json:"profit"`
	Items               []Item  `json:"items"`
}

// Tables are the three views the report can contain.
type Tables struct {
	Summary           report.Table
	Details           report.Table
	DetailsWithMods   report.Table
	HasDetailsWithMod bool
}

// Map builds all three tables; DetailsWithMods is meaningful only when some item carries modifiers.
func Map(data []Category) Tables {
	t := Tables{Summary: summary(data), Details: details(data)}
	t.DetailsWithMods, t.HasDetailsWithMod = detailsWithModifiers(data, t.Details)
	return t
}

func summary(data []Category) report.Table {
	rows := make([]report.Row, 0, len(data))
	for _, c := range data {
		rows = append(rows, report.Row{
			"category": c.DepartmentNameAlias, "price": avg(c.GrossSales, c.Quantity),
			"quantity": c.Quantity, "gross": c.GrossSales, "discount": c.DiscountAmount,
			"cashDiscountAmt": c.CashDiscountAmount, "coinDiscountAmt": c.CoinDiscountAmount,
			"netSales": c.NetSales, "tax": c.TaxAmount, "merchantFee": c.MerchantFee,
			"cost": c.Cost, "profit": c.Profit,
		})
	}
	return report.Table{
		Title: "Category Summary",
		Labels: []report.Label{
			{Prop: "category", Name: "Category", Type: "string"},
			{Prop: "price", Name: "Avg Price", Type: "currency"},
			{Prop: "quantity", Name: "Item Qty", Type: "int"},
			{Prop: "gross", Name: "Gross Amount", Type: "currency"},
			{Prop: "discount", Name: "Discount Amount", Type: "currency"},
			{Prop: "cashDiscountAmt", Name: "Cash Discount", Type: "currency"},
			{Prop: "coinDiscountAmt", Name: "Coin Discount", Type: "currency"},
			{Prop: "netSales", Name: "Net Sales", Type: "currency"},
			{Prop: "tax", Name: "Tax", Type: "currency"},
			{Prop: "merchantFee", Name: "Merchant Fee", Type: "currency"},
			{Prop: "cost", Name: "Cost", Type: "currency"},
			{Prop: "profit", Name: "Profit", Type: "currency"},
		},
		Rows: rows,
	}
}

func details(data []Category) report.Table {
	var tot struct {
		quantity, gross, discount, cashDisc, coinDisc, net, tax, fee, cost, profit float64
	}
	// The grand total's price column sums every category's and item's average.
	totalPrice := 0.0
	for _, c := range data {
		tot.quantity += c.Quantity
		tot.gross += c.GrossSales
		tot.discount += c.DiscountAmount
		tot.cashDisc += c.CashDiscountAmount
		tot.coinDisc += c.CoinDiscountAmount
		tot.net += c.NetSales
		tot.tax += c.TaxAmount
		tot.fee += c.MerchantFee
		tot.cost += c.Cost
		tot.profit += c.Profit
		totalPrice += avg(c.GrossSales, c.Quantity)
		for _, it := range c.Items {
			totalPrice += avg(it.GrossSales, it.Quantity)
		}
	}

	rows := []report.Row{{
		"category": "ALL CATEGORIES", "item": "ALL ITEMS", "price": totalPrice,
		"quantity": tot.quantity, "grossSales": tot.gross, "discountAmt": tot.discount,
		"cashDiscountAmt": tot.cashDisc, "coinDiscountAmt": tot.coinDisc, "netSales": tot.net,
		"taxAmt": tot.tax, "merchantFee": tot.fee, "cost": tot.cost, "profit": tot.profit,
		"qtyPercentGroup": "", "qtyPercentAll": "", "netAmtPercentGroup": "", "netAmtPercentAll": "",
		"rowOptions": map[string]any{"bold": true},
	}}

	for _, c := range data {
		rows = append(rows, report.Row{
			"category": c.DepartmentNameAlias, "item": "All Items", "price": avg(c.GrossSales, c.Quantity),
			"quantity": c.Quantity, "grossSales": c.GrossSales, "discountAmt": c.DiscountAmount,
			"cashDiscountAmt": c.CashDiscountAmount, "coinDiscountAmt": c.CoinDiscountAmount,
			"netSales": c.NetSales, "taxAmt": c.TaxAmount, "merchantFee": c.MerchantFee,
			"cost": c.Cost, "profit": c.Profit,
			"qtyPercentGroup": 1.0, "qtyPercentAll": share(c.Quantity, tot.quantity),
			"netAmtPercentGroup": 1.0, "netAmtPercentAll": share(c.NetSales, tot.net),
			"rowOptions": map[string]any{"bold": true},
		})
		for _, it := range c.Items {
			rows = append(rows, report.Row{
				"category": "", "item": text(it.Description), "price": avg(it.GrossSales, it.Quantity),
				"quantity": it.Quantity, "grossSales": it.GrossSales, "discountAmt": it.DiscountAmount,
				"cashDiscountAmt": it.CashDiscountAmount, "coinDiscountAmt": it.CoinDiscountAmount,
				"netSales": it.NetSales, "taxAmt": it.TaxAmount, "merchantFee": it.MerchantFee,
				"cost": it.Cost, "profit": it.Profit,
				"qtyPercentGroup": share(it.Quantity, c.Quantity), "qtyPercentAll": share(it.Quantity, tot.quantity),
				"netAmtPercentGroup": share(it.NetSales, c.NetSales), "netAmtPercentAll": share(it.NetSales, tot.net),
			})
		}
	}

	return report.Table{Title: "Item Details", Labels: detailLabels(), Rows: rows}
}

// Modifier rows fill only quantity and price; the rest are blank on purpose.
func detailsWithModifiers(data []Category, base report.Table) (report.Table, bool) {
	hasMods := false
	for _, c := range data {
		for _, it := range c.Items {
			if len(it.Modifiers) > 0 {
				hasMods = true
			}
		}
	}
	if !hasMods {
		return report.Table{}, false
	}

	rows := make([]report.Row, 0, len(base.Rows))
	cursor := 0
	next := func() report.Row {
		r := report.Row{}
		maps.Copy(r, base.Rows[cursor])
		r["modifier"] = ""
		cursor++
		return r
	}
	rows = append(rows, next())
	for _, c := range data {
		rows = append(rows, next())
		for _, it := range c.Items {
			rows = append(rows, next())
			for _, mod := range it.Modifiers {
				price := mod.AvgPrice
				if price == nil {
					p := avg(mod.GrossSales, mod.Quantity)
					price = &p
				}
				rows = append(rows, report.Row{
					"category": "", "item": "", "modifier": text(mod.Description),
					"price": *price, "quantity": mod.Quantity, "grossSales": mod.GrossSales,
					"discountAmt": "", "cashDiscountAmt": "", "coinDiscountAmt": "", "netSales": "",
					"taxAmt": "", "merchantFee": "", "cost": "", "profit": "",
					"qtyPercentGroup": "", "qtyPercentAll": "", "netAmtPercentGroup": "", "netAmtPercentAll": "",
				})
			}
		}
	}

	labels := detailLabels()
	labels = append(labels[:2], append([]report.Label{
		{Prop: "modifier", Name: "Modifier", Type: "string"},
	}, labels[2:]...)...)
	return report.Table{Title: "Item Details with Modifier", Labels: labels, Rows: rows}, true
}

func detailLabels() []report.Label {
	return []report.Label{
		{Prop: "category", Name: "Sales Category", Type: "string"},
		{Prop: "item", Name: "Item", Type: "string"},
		{Prop: "price", Name: "Avg Price", Type: "currency"},
		{Prop: "quantity", Name: "Quantity", Type: "int"},
		{Prop: "grossSales", Name: "Gross Sales", Type: "currency"},
		{Prop: "discountAmt", Name: "Discount Amount", Type: "currency"},
		{Prop: "cashDiscountAmt", Name: "Cash Discount", Type: "currency"},
		{Prop: "coinDiscountAmt", Name: "Coin Discount", Type: "currency"},
		{Prop: "netSales", Name: "Net Sales", Type: "currency"},
		{Prop: "taxAmt", Name: "Tax", Type: "currency"},
		{Prop: "merchantFee", Name: "Merchant Fee", Type: "currency"},
		{Prop: "cost", Name: "Cost", Type: "currency"},
		{Prop: "profit", Name: "Profit", Type: "currency"},
		{Prop: "qtyPercentGroup", Name: "% Qty (Category)", Type: "percent"},
		{Prop: "qtyPercentAll", Name: "% Qty (All)", Type: "percent"},
		{Prop: "netAmtPercentGroup", Name: "% Net Amt (Category)", Type: "percent"},
		{Prop: "netAmtPercentAll", Name: "% Net Amt (All)", Type: "percent"},
	}
}

func avg(gross, quantity float64) float64 {
	if quantity == 0 {
		return 0
	}
	return math.Round(gross/quantity*100) / 100
}

// Node defaults a missing total to 1.
func share(value, total float64) float64 {
	if total == 0 {
		total = 1
	}
	return value / total
}

func text(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
