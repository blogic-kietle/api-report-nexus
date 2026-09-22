package pdf

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

// Datasets is the table payload an export-pdf request may carry instead of HTML.
type Datasets struct {
	Meta   *StoreMeta `json:"meta"`
	Tables Tables     `json:"tables"`
}

// StoreMeta is the letterhead shown above the tables.
type StoreMeta struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Phone   string   `json:"phone"`
	Dates   []string `json:"dates"`
	// Date is filled in from Dates before rendering.
	Date string `json:"date"`
}

type Table struct {
	Key    string   `json:"-"`
	Title  string   `json:"title"`
	Labels []Label  `json:"labels"`
	Rows   [][]Cell `json:"datasets"`
}

// Label is a column header; the Is* methods drive the template's width and alignment classes.
type Label struct {
	Prop string `json:"prop"`
	Name string `json:"name"`
	Type string `json:"type"`
	Bold bool   `json:"bold"`
}

func (l Label) IsString() bool    { return l.Type == "string" }
func (l Label) IsNotString() bool { return l.Type != "string" }
func (l Label) IsNumberOrCurrency() bool {
	return l.Type == "number" || l.Type == "currency" || l.Type == "int"
}
func (l Label) IsPropDateOrStartDate() bool {
	return l.Prop == "Date" || l.Prop == "StartDate" || l.Type == "date"
}
func (l Label) IsPropSummaryOrDates() bool { return l.Prop == "Summary" || l.Prop == "Dates" }

type Cell struct {
	Value       any    `json:"value"`
	IsTextRight bool   `json:"isTextRight"`
	IsBold      bool   `json:"isBold"`
	Link        string `json:"link"`
}

// Display renders a number as JavaScript would, so 1000000 does not become 1e+06.
func (c Cell) Display() any {
	if f, ok := c.Value.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return c.Value
}

// Tables keeps the payload's key order, which the render follows and a map would lose.
type Tables []Table

// UnmarshalJSON accepts either an object keyed by table name or a plain array.
func (t *Tables) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return errors.New("pdf: tables must be an object or an array")
	}
	if delim == '[' {
		var arr []Table
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		*t = arr
		return nil
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		var table Table
		if err := dec.Decode(&table); err != nil {
			return err
		}
		table.Key, _ = key.(string)
		*t = append(*t, table)
	}
	return nil
}

// RowCount decides whether the render is split over several pages.
func (t Tables) RowCount() int {
	n := 0
	for _, table := range t {
		n += len(table.Rows)
	}
	return n
}

// View is what the general-table template renders.
type View struct {
	ShowHeader bool
	Store      *StoreMeta
	Title      string
	Tables     Tables
}
