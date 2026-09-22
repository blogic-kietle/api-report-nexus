package report

import "math"

// Label describes one column; the JSON tags match the Node mapper output so goldens decode straight into it.
type Label struct {
	Prop           string `json:"prop"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Format         string `json:"format,omitempty"`
	Note           string `json:"note,omitempty"`
	Bold           bool   `json:"bold,omitempty"`
	Width          int    `json:"width,omitempty"`
	CalculateTotal bool   `json:"calculateTotal,omitempty"`
}

// TotalRow reports whether a row is a summary line: the isTotal flag, or "Summary"/"Total" in a named column.
func TotalRow(fields ...string) func(Row) bool {
	return func(r Row) bool {
		if flag, ok := r["isTotal"].(bool); ok && flag {
			return true
		}
		for _, f := range fields {
			if s, ok := r[f].(string); ok && (s == "Summary" || s == "Total") {
				return true
			}
		}
		return false
	}
}

// Row is one record; the mappers emit ~44 differently shaped sections, so it stays a bag of values.
type Row map[string]any

type Table struct {
	Title       string  `json:"title,omitempty"`
	Orientation string  `json:"orientation,omitempty"`
	Labels      []Label `json:"labels"`
	Rows        []Row   `json:"datasets"`
}

// Horizontal reports whether the table is laid out label-by-row.
func (t Table) Horizontal() bool { return t.Orientation == "horizontal" }

type Meta struct {
	Title     string
	StoreID   string
	StoreName string
	// ISO-8601 with offset, exactly as the POS sends them.
	From, To string
}

// Config is one POS configuration pair, e.g. "SaleTypesConfig.ToGoLabel" -> "Take Out".
type Config struct {
	Name  string `json:"configurationName"`
	Value string `json:"configurationValue"`
}

// Percent is value/total rounded to 6 places (NumberHelper.calcPercent), or 0 without a total.
func Percent(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(value/total*1e6) / 1e6
}
