package daypart

import (
	"sort"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/pkg/datetime"
)

// Shift is one POS record: a day part on one date.
type Shift struct {
	DayOfWeek          string  `json:"dayOfWeek"`
	DayPart            string  `json:"dayPart"`
	StartDate          string  `json:"startDate"`
	EndDate            string  `json:"endDate"`
	DineInSales        float64 `json:"dineInSales"`
	ToGoSales          float64 `json:"toGoSales"`
	OtherSales         float64 `json:"otherSales"`
	AccountReceivables float64 `json:"accountReceivables"`
	TicketsCount       float64 `json:"ticketsCount"`
	GuestsCount        float64 `json:"guestsCount"`
}

var weekdays = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

// Map groups shifts by weekday then day part and totals each group; shifts outside [from, to] are dropped.
func Map(shifts []Shift, from, to string, configs []report.Config) []report.Table {
	shifts = inRange(shifts, from, to)
	if len(shifts) == 0 {
		return nil
	}
	return Build(shifts, SortedDays(shifts), configs)
}

// Build makes one table per listed weekday, in that order; the Sales Summary reuses it with its own weekdays.
func Build(shifts []Shift, days []string, configs []report.Config) []report.Table {
	dineIn, toGo := Labels(configs)

	tables := make([]report.Table, 0, len(days))
	for _, day := range days {
		t := report.Table{
			// yes, a single space: Node writes it into column A of the header row
			Title: " ",
			Labels: []report.Label{
				{Prop: "dayPart", Name: day, Type: "string", Width: 25},
				{Prop: "dineInSales", Name: dineIn + " Sales", Type: "currency", Note: "Taxes and service charges not included"},
				{Prop: "toGoSales", Name: toGo + " Sales", Type: "currency", Note: "Taxes and service charges not included"},
				{Prop: "otherSales", Name: "Other Sales", Type: "currency", Note: "Taxes and service charges not included"},
				{Prop: "accountReceivables", Name: "Account Receivables", Type: "currency", Note: "Taxes and service charges not included"},
				{Prop: "totalNetSales", Name: "Total Net Sales", Type: "currency"},
				{Prop: "ticketsCount", Name: "Tickets Count", Type: "number"},
				{Prop: "guestsCount", Name: "Guests Count", Type: "number"},
				{Prop: "percentage", Name: "Percentage", Type: "percent"},
			},
		}

		var minutes []int
		var total float64
		for _, part := range uniqueParts(shifts, day) {
			var acc Shift
			label, tod := "", 0
			for _, s := range shifts {
				if s.DayOfWeek != day || s.DayPart != part {
					continue
				}
				// Node overwrites these every iteration, so the last shift wins.
				start, _ := datetime.ParseISO(s.StartDate)
				end, _ := datetime.ParseISO(s.EndDate)
				label = part + " " + start.Format("03:04 PM") + " - " + end.Format("03:04 PM")
				tod = start.Hour()*60 + start.Minute()
				acc.DineInSales += s.DineInSales
				acc.ToGoSales += s.ToGoSales
				acc.OtherSales += s.OtherSales
				acc.AccountReceivables += s.AccountReceivables
				acc.TicketsCount += s.TicketsCount
				acc.GuestsCount += s.GuestsCount
			}
			net := acc.DineInSales + acc.ToGoSales + acc.OtherSales + acc.AccountReceivables
			total += net
			t.Rows = append(t.Rows, report.Row{
				"dayPart":            label,
				"dineInSales":        acc.DineInSales,
				"toGoSales":          acc.ToGoSales,
				"otherSales":         acc.OtherSales,
				"accountReceivables": acc.AccountReceivables,
				"totalNetSales":      net,
				"ticketsCount":       acc.TicketsCount,
				"guestsCount":        acc.GuestsCount,
			})
			minutes = append(minutes, tod)
		}

		// Order rows by time of day (date discarded), stable like Array.sort.
		idx := make([]int, len(t.Rows))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool { return minutes[idx[a]] < minutes[idx[b]] })
		rows := make([]report.Row, len(idx))
		for i, j := range idx {
			rows[i] = t.Rows[j]
			rows[i]["percentage"] = report.Percent(rows[i]["totalNetSales"].(float64), total)
		}
		t.Rows = rows
		tables = append(tables, t)
	}
	return tables
}

// Unparseable dates are kept, as in Node.
func inRange(shifts []Shift, from, to string) []Shift {
	f, okF := datetime.ParseISO(from)
	t, okT := datetime.ParseISO(to)
	var out []Shift
	for _, s := range shifts {
		start, okS := datetime.ParseISO(s.StartDate)
		end, okE := datetime.ParseISO(s.EndDate)
		if okF && okE && end.Before(f) {
			continue
		}
		if okT && okS && start.After(t) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// SortedDays returns the distinct weekdays present, Monday first; unknown names sort first.
func SortedDays(shifts []Shift) []string {
	var days []string
	seen := map[string]bool{}
	for _, s := range shifts {
		if !seen[s.DayOfWeek] {
			seen[s.DayOfWeek] = true
			days = append(days, s.DayOfWeek)
		}
	}
	sort.SliceStable(days, func(a, b int) bool { return DayIndex(days[a]) < DayIndex(days[b]) })
	return days
}

// DayIndex is the weekday's position, Monday = 0, or -1 for an unknown name.
func DayIndex(name string) int {
	for i, d := range weekdays {
		if d == name {
			return i
		}
	}
	return -1
}

func uniqueParts(shifts []Shift, day string) []string {
	var parts []string
	seen := map[string]bool{}
	for _, s := range shifts {
		if s.DayOfWeek == day && !seen[s.DayPart] {
			seen[s.DayPart] = true
			parts = append(parts, s.DayPart)
		}
	}
	return parts
}

// Labels resolves the store's names for the two sale types.
func Labels(configs []report.Config) (dineIn, toGo string) {
	dineIn, toGo = "Dine In", "To Go"
	for _, c := range configs {
		switch c.Name {
		case "SaleTypesConfig.DineInLabel":
			if c.Value != "" {
				dineIn = c.Value
			}
		case "SaleTypesConfig.ToGoLabel":
			if c.Value != "" {
				toGo = c.Value
			}
		}
	}
	return dineIn, toGo
}
