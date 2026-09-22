package sendmail

import (
	"context"
	"encoding/json"
	"time"

	"api-report-nexus/internal/domain/category"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/infrastructure/excel"
	"api-report-nexus/internal/pkg/datetime"
)

type CategoryData struct {
	GroupByCategory struct {
		CategoryData []category.Category `json:"categoryData"`
	} `json:"groupByCategory"`
}

func boldRow(r report.Row) bool {
	opts, ok := r["rowOptions"].(map[string]any)
	if !ok {
		return false
	}
	bold, _ := opts["bold"].(bool)
	return bold
}

// SalesByCategoryWorkbook writes the summary sheet plus the item and modifier sheets the store has switched on.
func SalesByCategoryWorkbook(d CategoryData, m report.Meta, withItems, withModifiers bool, now time.Time) ([]byte, error) {
	tables := category.Map(d.GroupByCategory.CategoryData)

	book, err := excel.New(now)
	if err != nil {
		return nil, err
	}
	sheet := func(name, title string, t report.Table) error {
		g, err := book.Grid(name)
		if err != nil {
			return err
		}
		meta := m
		meta.Title = title
		if err := g.Meta(meta, nil, now); err != nil {
			return err
		}
		return g.Table(t, excel.TableOpts{TotalRow: boldRow})
	}

	summary := tables.Summary
	for i := range summary.Labels {
		summary.Labels[i].CalculateTotal = true
	}
	if err := sheet(tables.Summary.Title, m.Title, summary); err != nil {
		return nil, err
	}
	if withItems {
		if err := sheet(tables.Details.Title, "Item Details Report", tables.Details); err != nil {
			return nil, err
		}
	}
	if withModifiers && tables.HasDetailsWithMod {
		if err := sheet(tables.DetailsWithMods.Title, "Item Details with Modifier Report", tables.DetailsWithMods); err != nil {
			return nil, err
		}
	}
	return book.Bytes()
}

// CategoryOption reads one SystemConfigValue.SalesByCategoryReport.* switch from the configurations JSON string; anything but "0" is on.
func CategoryOption(configs, key string) bool {
	var m map[string]string
	if configs == "" {
		configs = "{}"
	}
	if err := json.Unmarshal([]byte(configs), &m); err != nil {
		// Node's JSON.parse failure fell back to an empty object
		return true
	}
	return m["SystemConfigValue.SalesByCategoryReport."+key] != "0"
}

func SalesByCategory(ctx context.Context, s email.Sender, r Request, configs string, d CategoryData, now time.Time) error {
	const title = "Sales By Category Report"
	data, err := SalesByCategoryWorkbook(d, r.Meta.report(title),
		CategoryOption(configs, "IncludeItemDetails"), CategoryOption(configs, "IncludeItemDetailsWithModifier"), now)
	if err != nil {
		return err
	}
	note, err := body(r.Meta.StoreName, title, r.Meta.FromDate, r.Meta.ToDate)
	if err != nil {
		return err
	}
	// This report spells its period with an abbreviated month, unlike the others.
	span := func(layout, sep string) string {
		f, okF := datetime.ParseISO(r.Meta.FromDate)
		t, okT := datetime.ParseISO(r.Meta.ToDate)
		if !okF || !okT {
			return ""
		}
		return f.Format(layout) + sep + t.Format(layout)
	}
	return s.Send(ctx, email.Message{
		ToEmails:    r.Emails,
		Subject:     r.Meta.StoreName + " - " + title + " - from " + span("Jan/02/2006 03:04 PM", " to "),
		Body:        note,
		FileName:    "Sales-By-Category-Report-from-" + span("Jan-02-2006-03-04-PM", "-to-") + ".xlsx",
		FileContent: attachExcel(data),
	})
}
