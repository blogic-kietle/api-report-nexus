package sendmail

import (
	"context"
	"strings"
	"text/template"
	"time"

	"api-report-nexus/assets"
	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/infrastructure/excel"
	"api-report-nexus/internal/pkg/datetime"
	"api-report-nexus/internal/pkg/money"
)

// text/template, not html/template: values go through mustacheEscape so the bytes match mustache.js.
var dayPartEmail = template.Must(template.New("sales-day-part-email.html").
	Funcs(template.FuncMap{"currency": money.USD}).
	ParseFS(assets.FS, "templates/sales-day-part-email.html"))

// mustacheEscape reproduces mustache.js, which also escapes "/" and uses hex entities.
var mustacheEscape = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;",
	"'", "&#39;", "/", "&#x2F;", "`", "&#x60;", "=", "&#x3D;",
).Replace

type dayPartView struct {
	Title  string
	Dates  string
	Config struct{ DineIn, ToGo string }
	Tables []dayPartTable
}

type dayPartTable struct {
	Title string
	Rows  []dayPartRow
}

type dayPartRow struct {
	DayPart                                                               string
	DineInSales, ToGoSales, OtherSales, AccountReceivables, TotalNetSales float64
	TicketsCount, GuestsCount                                             float64
}

func SalesDayPartWorkbook(tables []report.Table, m report.Meta, now time.Time) ([]byte, error) {
	return build(m, now, func(g *excel.Grid) error {
		for _, t := range tables {
			if err := g.Table(t, excel.TableOpts{Title: t.Title}); err != nil {
				return err
			}
		}
		return nil
	})
}

func dayPartBody(tables []report.Table, m report.Meta, configs []report.Config) (string, error) {
	view := dayPartView{
		Title: mustacheEscape(m.Title),
		Dates: mustacheEscape(datetime.FormatDates(m.From, m.To)),
	}
	dineIn, toGo := daypart.Labels(configs)
	view.Config.DineIn, view.Config.ToGo = mustacheEscape(dineIn), mustacheEscape(toGo)
	for _, t := range tables {
		out := dayPartTable{Title: mustacheEscape(t.Title)}
		for _, r := range t.Rows {
			out.Rows = append(out.Rows, dayPartRow{
				DayPart:            mustacheEscape(str(r["dayPart"])),
				DineInSales:        num(r["dineInSales"]),
				ToGoSales:          num(r["toGoSales"]),
				OtherSales:         num(r["otherSales"]),
				AccountReceivables: num(r["accountReceivables"]),
				TotalNetSales:      num(r["totalNetSales"]),
				TicketsCount:       num(r["ticketsCount"]),
				GuestsCount:        num(r["guestsCount"]),
			})
		}
		view.Tables = append(view.Tables, out)
	}
	var b strings.Builder
	if err := dayPartEmail.Execute(&b, view); err != nil {
		return "", err
	}
	return b.String(), nil
}

func SalesDayPart(ctx context.Context, s email.Sender, r Request, shifts []daypart.Shift, configs []report.Config, now time.Time) error {
	const title = "Sales by Day Part Report"
	meta := r.Meta.report(title)
	tables := daypart.Map(shifts, r.Meta.FromDate, r.Meta.ToDate, configs)

	data, err := SalesDayPartWorkbook(tables, meta, now)
	if err != nil {
		return err
	}
	note, err := dayPartBody(tables, meta, configs)
	if err != nil {
		return err
	}
	return s.Send(ctx, excelMail(r, title, note, "Sales-by-Day-Part-Report"+fileStamp(r.Meta.FromDate, r.Meta.ToDate)+".xlsx", data))
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}
