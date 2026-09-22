package sendmail

import (
	"context"
	"strings"
	"text/template"
	"time"

	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/domain/salessummary"
	"api-report-nexus/internal/infrastructure/excel"
)

// summaryBody is the standard note with different indentation on the first and last lines, kept byte for byte.
var summaryBody = template.Must(template.New("summary").Parse("\n        " + noteHead +
	`            <p>Attached to this email, you will find the Sales Summary Report covering the period from {{.Period}}.</p>
` + noteTail + "\n        "))

// Heading rows the mapper flagged: bold without the double rule.
func isBoldRow(r report.Row) bool {
	b, _ := r["isBold"].(bool)
	return b
}

// SalesSummaryWorkbook puts every section on "Summary" except the hourly, weekday and day part breakdowns, one sheet each.
func SalesSummaryWorkbook(in salessummary.Input, m report.Meta, o salessummary.Options, now time.Time) ([]byte, error) {
	sections := salessummary.Map(in, o)

	book, err := excel.New(now)
	if err != nil {
		return nil, err
	}
	summary, err := book.Grid("Summary")
	if err != nil {
		return nil, err
	}
	if err := summary.Meta(m, nil, now); err != nil {
		return nil, err
	}

	totalRule := report.TotalRow("name", "reason")
	var hourly, weekday report.Table
	shown := 0
	for _, s := range sections {
		switch s.Key {
		case "hourlyBreakdown":
			hourly = s.Table
			continue
		case "weekdayBreakdown":
			weekday = s.Table
			continue
		}
		if len(s.Rows) == 0 {
			continue
		}
		start := 1
		if shown == 3 {
			// Node indents the fourth populated section; parity, not design
			start = 3
		}
		if err := summary.Table(s.Table, excel.TableOpts{
			Title: s.Title, StartCol: start, TotalRow: totalRule, Bold: isBoldRow,
		}); err != nil {
			return nil, err
		}
		summary.BlankRow()
		shown++
	}

	// Breakdown sheets carry their name as the report title and no table title.
	for _, x := range []struct {
		name  string
		table report.Table
	}{{"Hourly Breakdown", hourly}, {"Weekday Breakdown", weekday}} {
		if len(x.table.Rows) == 0 {
			continue
		}
		g, err := book.Grid(x.name)
		if err != nil {
			return nil, err
		}
		meta := m
		meta.Title = x.name
		if err := g.Meta(meta, nil, now); err != nil {
			return nil, err
		}
		if err := g.Table(x.table, excel.TableOpts{TotalRow: report.TotalRow("title")}); err != nil {
			return nil, err
		}
	}

	// Node created this sheet whenever the key was present, even for an empty list.
	if in.SalesByDayPartBreakdown != nil {
		g, err := book.Grid("Day Part Breakdown")
		if err != nil {
			return nil, err
		}
		meta := m
		meta.Title = "Sales by Day Part Report"
		if err := g.Meta(meta, nil, now); err != nil {
			return nil, err
		}
		for _, t := range salessummary.DayPartBreakdown(in.SalesByDayPartBreakdown, m.From, m.To, in.LabelConfigs) {
			if err := g.Table(t, excel.TableOpts{Title: t.Title, TotalRow: totalRule}); err != nil {
				return nil, err
			}
		}
	}
	return book.Bytes()
}

func SalesSummary(ctx context.Context, s email.Sender, r Request, in salessummary.Input, clientURL string, now time.Time) error {
	const title = "Sales Summary Report"
	data, err := SalesSummaryWorkbook(in, r.Meta.report(title), salessummary.Options{
		Version: r.Meta.POSVersion, StoreID: r.Meta.StoreID, ClientURL: clientURL,
	}, now)
	if err != nil {
		return err
	}
	var note strings.Builder
	if err := summaryBody.Execute(&note, struct{ Store, Period string }{
		Store: mustacheEscape(storeOrCustomer(r.Meta.StoreName)), Period: plainPeriod(r.Meta.FromDate, r.Meta.ToDate),
	}); err != nil {
		return err
	}
	return s.Send(ctx, excelMail(r, title, note.String(), "Sales-Summary-Report"+fileStamp(r.Meta.FromDate, r.Meta.ToDate)+".xlsx", data))
}
