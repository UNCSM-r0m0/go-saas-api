package runtime

import (
	"strings"
	"testing"
)

func TestDetectTemporalQuery_SpanishDay(t *testing.T) {
	if got := DetectTemporalQuery("qué día es hoy"); got != TemporalDay {
		t.Errorf("expected TemporalDay, got %d", got)
	}
}

func TestDetectTemporalQuery_SpanishDate(t *testing.T) {
	if got := DetectTemporalQuery("qué fecha es hoy"); got != TemporalDate {
		t.Errorf("expected TemporalDate, got %d", got)
	}
}

func TestDetectTemporalQuery_SpanishTime(t *testing.T) {
	if got := DetectTemporalQuery("qué hora es"); got != TemporalTime {
		t.Errorf("expected TemporalTime, got %d", got)
	}
}

func TestDetectTemporalQuery_SpanishAccents(t *testing.T) {
	cases := []struct {
		input string
		want  TemporalIntent
	}{
		{"qué día es hoy", TemporalDay},
		{"qué fecha es hoy", TemporalDate},
		{"qué hora es", TemporalTime},
		{"cuál es el día de hoy", TemporalDay},
		{"cuál es la fecha de hoy", TemporalDate},
		{"cuál es la hora", TemporalTime},
	}
	for _, c := range cases {
		if got := DetectTemporalQuery(c.input); got != c.want {
			t.Errorf("%q: expected %d, got %d", c.input, c.want, got)
		}
	}
}

func TestDetectTemporalQuery_EnglishDay(t *testing.T) {
	if got := DetectTemporalQuery("what day is it"); got != TemporalDay {
		t.Errorf("expected TemporalDay, got %d", got)
	}
	if got := DetectTemporalQuery("what day is today"); got != TemporalDay {
		t.Errorf("expected TemporalDay, got %d", got)
	}
}

func TestDetectTemporalQuery_EnglishDate(t *testing.T) {
	if got := DetectTemporalQuery("what is the date today"); got != TemporalDate {
		t.Errorf("expected TemporalDate, got %d", got)
	}
	if got := DetectTemporalQuery("what's today's date"); got != TemporalDate {
		t.Errorf("expected TemporalDate, got %d", got)
	}
}

func TestDetectTemporalQuery_EnglishTime(t *testing.T) {
	if got := DetectTemporalQuery("what time is it"); got != TemporalTime {
		t.Errorf("expected TemporalTime, got %d", got)
	}
	if got := DetectTemporalQuery("what's the time"); got != TemporalTime {
		t.Errorf("expected TemporalTime, got %d", got)
	}
}

func TestDetectTemporalQuery_NonTemporal(t *testing.T) {
	cases := []string{
		"hola, cómo estás",
		"escríbeme un script en python",
		"explain quantum computing",
		"qué opinas de golang",
		"dime un chiste",
	}
	for _, c := range cases {
		if got := DetectTemporalQuery(c); got != TemporalNone {
			t.Errorf("%q: expected TemporalNone, got %d", c, got)
		}
	}
}

func TestDetectTemporalQuery_CaseInsensitive(t *testing.T) {
	if got := DetectTemporalQuery("QUÉ DÍA ES HOY"); got != TemporalDay {
		t.Errorf("expected TemporalDay, got %d", got)
	}
	if got := DetectTemporalQuery("What Day Is It"); got != TemporalDay {
		t.Errorf("expected TemporalDay, got %d", got)
	}
}

func TestFormatTemporalResponse_Date(t *testing.T) {
	resp := FormatTemporalResponse(TemporalDate, "UTC")
	if !strings.HasPrefix(resp, "Hoy es ") {
		t.Errorf("expected Spanish date prefix, got: %s", resp)
	}
	if !strings.Contains(resp, "de") {
		t.Error("expected month particle 'de'")
	}
}

func TestFormatTemporalResponse_Day(t *testing.T) {
	resp := FormatTemporalResponse(TemporalDay, "UTC")
	if !strings.HasPrefix(resp, "Hoy es ") {
		t.Errorf("expected Spanish day prefix, got: %s", resp)
	}
	if !strings.HasSuffix(resp, ".") {
		t.Error("expected trailing period")
	}
}

func TestFormatTemporalResponse_Time(t *testing.T) {
	resp := FormatTemporalResponse(TemporalTime, "UTC")
	if !strings.HasPrefix(resp, "Son las ") {
		t.Errorf("expected Spanish time prefix, got: %s", resp)
	}
	if !strings.Contains(resp, "en UTC") {
		t.Errorf("expected timezone mention, got: %s", resp)
	}
}

func TestFormatTemporalResponse_InvalidTimezone(t *testing.T) {
	// Must not panic and must still produce output
	resp := FormatTemporalResponse(TemporalDate, "invalid-zone")
	if !strings.HasPrefix(resp, "Hoy es ") {
		t.Errorf("expected fallback to UTC without panic, got: %s", resp)
	}
}

func TestFormatTemporalResponse_EmptyTimezone(t *testing.T) {
	resp := FormatTemporalResponse(TemporalDate, "")
	if !strings.HasPrefix(resp, "Hoy es ") {
		t.Errorf("expected fallback to UTC, got: %s", resp)
	}
}
