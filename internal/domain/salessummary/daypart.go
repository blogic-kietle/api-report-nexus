package salessummary

import (
	"time"

	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/pkg/datetime"
)

// DayPartBreakdown keeps every shift but, for a period under a week, lists only the weekdays from its first day to its last.
func DayPartBreakdown(shifts []daypart.Shift, from, to string, configs []report.Config) []report.Table {
	if len(shifts) == 0 {
		return nil
	}
	days := daypart.SortedDays(shifts)

	start, okS := datetime.ParseISO(from)
	end, okE := datetime.ParseISO(to)
	if okS && okE {
		start = startOfDay(start)
		end = startOfDay(end).Add(24*time.Hour - time.Millisecond)
		if end.Sub(start).Hours()/24 < 7 {
			// Positions counted from the period's first weekday, 1-based.
			pos := func(day string) int {
				i := daypart.DayIndex(day)
				if i < 0 {
					// unknown names fall outside every range
					return 0
				}
				return (i-weekday(start)+7)%7 + 1
			}
			last := pos(weekdayName(end))
			var kept []string
			for _, d := range days {
				if p := pos(d); p >= 1 && p <= last {
					kept = append(kept, d)
				}
			}
			days = kept
		}
	}
	if len(days) == 0 {
		return nil
	}
	return daypart.Build(shifts, days, configs)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// weekday is Monday = 0 … Sunday = 6, matching daypart.DayIndex.
func weekday(t time.Time) int { return (int(t.Weekday()) + 6) % 7 }

func weekdayName(t time.Time) string {
	return []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}[weekday(t)]
}
