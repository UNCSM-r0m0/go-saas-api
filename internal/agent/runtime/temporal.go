package runtime

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// TemporalIntent represents the type of temporal query detected.
type TemporalIntent int

const (
	TemporalNone TemporalIntent = iota
	TemporalDate
	TemporalDay
	TemporalTime
)

var (
	spanishDayPatterns = []string{
		`qu[eé]\s+d[ií]a\s+es\s+hoy`,
		`cu[aá]l\s+es\s+el\s+d[ií]a\s+(de\s+hoy|hoy)`,
	}
	spanishDatePatterns = []string{
		`qu[eé]\s+fecha\s+es\s+hoy`,
		`cu[aá]l\s+es\s+la\s+fecha\s+(de\s+hoy|hoy)`,
	}
	spanishTimePatterns = []string{
		`qu[eé]\s+hora\s+es`,
		`cu[aá]l\s+es\s+la\s+hora`,
	}

	englishDayPatterns = []string{
		`what\s+day\s+is\s+(it|today)`,
		`what's\s+today's\s+day`,
	}
	englishDatePatterns = []string{
		`what\s+is\s+the\s+date\s+today`,
		`what's\s+today's\s+date`,
		`what\s+date\s+is\s+it\s+today`,
	}
	englishTimePatterns = []string{
		`what\s+time\s+is\s+it`,
		`what's\s+the\s+time`,
	}
)

// DetectTemporalQuery analyzes the user content and returns the detected temporal intent.
func DetectTemporalQuery(content string) TemporalIntent {
	normalized := normalize(content)

	for _, p := range spanishDayPatterns {
		if matchPattern(normalized, p) {
			return TemporalDay
		}
	}
	for _, p := range spanishDatePatterns {
		if matchPattern(normalized, p) {
			return TemporalDate
		}
	}
	for _, p := range spanishTimePatterns {
		if matchPattern(normalized, p) {
			return TemporalTime
		}
	}

	for _, p := range englishDayPatterns {
		if matchPattern(normalized, p) {
			return TemporalDay
		}
	}
	for _, p := range englishDatePatterns {
		if matchPattern(normalized, p) {
			return TemporalDate
		}
	}
	for _, p := range englishTimePatterns {
		if matchPattern(normalized, p) {
			return TemporalTime
		}
	}

	return TemporalNone
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = stripAccents(s)
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	return s
}

func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 'á':
			b.WriteRune('a')
		case 'é':
			b.WriteRune('e')
		case 'í':
			b.WriteRune('i')
		case 'ó':
			b.WriteRune('o')
		case 'ú':
			b.WriteRune('u')
		case 'ü':
			b.WriteRune('u')
		case 'ñ':
			b.WriteRune('n')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func matchPattern(s, pattern string) bool {
	re := regexp.MustCompile(`^(.*\s)?` + pattern + `(\s.*)?$`)
	return re.MatchString(s)
}

var spanishMonths = map[time.Month]string{
	time.January:   "enero",
	time.February:  "febrero",
	time.March:     "marzo",
	time.April:     "abril",
	time.May:       "mayo",
	time.June:      "junio",
	time.July:      "julio",
	time.August:    "agosto",
	time.September: "septiembre",
	time.October:   "octubre",
	time.November:  "noviembre",
	time.December:  "diciembre",
}

var spanishWeekdays = map[time.Weekday]string{
	time.Sunday:    "domingo",
	time.Monday:    "lunes",
	time.Tuesday:   "martes",
	time.Wednesday: "miércoles",
	time.Thursday:  "jueves",
	time.Friday:    "viernes",
	time.Saturday:  "sábado",
}

// FormatTemporalResponse builds a natural Spanish response for the given temporal intent.
func FormatTemporalResponse(intent TemporalIntent, timezone string) string {
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	weekday := spanishWeekdays[now.Weekday()]
	month := spanishMonths[now.Month()]

	switch intent {
	case TemporalDate:
		return fmt.Sprintf("Hoy es %s, %d de %s de %d.", weekday, now.Day(), month, now.Year())
	case TemporalDay:
		return fmt.Sprintf("Hoy es %s.", weekday)
	case TemporalTime:
		return fmt.Sprintf("Son las %02d:%02d en %s.", now.Hour(), now.Minute(), timezone)
	default:
		return ""
	}
}
