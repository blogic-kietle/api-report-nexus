package datetime

import (
	"strconv"
	"strings"
	"time"
)

const (
	dateShort = "01/2/2006"
	datePad   = "01/02/2006"
	clock     = "03:04 PM"
	fileDate  = "01-02-2006"
	fileStamp = "01-02-2006 03-04-PM"
)

func ParseISO(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FormatDates prints the period as "09/1/2026 12:00 AM - 09/7/2026 11:59 PM", or one date with two clocks when both ends fall on the same day.
func FormatDates(from, to string) string {
	f, ok := ParseISO(from)
	if !ok {
		return ""
	}
	t, ok := ParseISO(to)
	if !ok {
		return f.Format(dateShort)
	}
	if sameDay(f, t) {
		return f.Format(dateShort) + " " + f.Format(clock) + " - " + t.Format(clock)
	}
	return f.Format(dateShort+" "+clock) + " - " + t.Format(dateShort+" "+clock)
}

// FormatDatesFull is the Grid variant: zero-padded day, never collapsed to a single date.
func FormatDatesFull(from, to string) string {
	f, ok := ParseISO(from)
	if !ok {
		return ""
	}
	t, ok := ParseISO(to)
	if !ok {
		return f.Format(fileStamp)
	}
	return f.Format(datePad+" "+clock) + " - " + t.Format(datePad+" "+clock)
}

// GenerateFileName is the download name: hyphenated title plus the date range (generateFileName).
func GenerateFileName(title, from, to string) string {
	if title == "" {
		return "export"
	}
	name := strings.ReplaceAll(title, " ", "-")
	f, ok := ParseISO(from)
	if !ok {
		return name
	}
	t, ok := ParseISO(to)
	if !ok {
		return name + " " + f.Format(fileDate)
	}
	return name + " From " + f.Format(fileStamp) + " to " + t.Format(fileStamp)
}

// FormatHour turns a POS hour bucket ("0".."23") into "3 PM"; unparseable input passes through, as in Node.
func FormatHour(h string) string {
	n, err := strconv.Atoi(h)
	if err != nil || n < 0 || n > 23 {
		return h
	}
	return time.Date(2000, 1, 1, n, 0, 0, 0, time.UTC).Format("3 PM")
}

// sameDay compares in a's zone: luxon's hasSame() converts the other side first.
func sameDay(a, b time.Time) bool {
	b = b.In(a.Location())
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
