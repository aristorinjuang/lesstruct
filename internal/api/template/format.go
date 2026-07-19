package template

import (
	"fmt"
	"time"
)

// LocalizedMonths maps language codes to their month names. When a language
// code is present, FormatDate and FormatDateTime use it; otherwise Go's
// native English formatting is used.
var LocalizedMonths = map[string][12]string{
	"id": {"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"},
}

// FormatDate returns a human-readable date string for the given time.
// When lang matches a key in LocalizedMonths, the localized month name is used.
// Otherwise Go's native English formatting ("January 2, 2006") is used.
func FormatDate(lang string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if months, ok := LocalizedMonths[lang]; ok {
		return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
	}
	return t.Format("January 2, 2006")
}

// FormatDateTime returns a human-readable date+time string for the given time.
// When lang matches a key in LocalizedMonths, the localized month name is used.
// Otherwise Go's native English formatting ("January 2, 2006 at 3:04 PM") is used.
func FormatDateTime(lang string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if _, ok := LocalizedMonths[lang]; ok {
		return fmt.Sprintf("%d %s %d", t.Day(), LocalizedMonths[lang][t.Month()-1], t.Year())
	}
	return t.Format("January 2, 2006 at 3:04 PM")
}
