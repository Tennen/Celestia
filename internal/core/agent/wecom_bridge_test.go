package agent

import "testing"

func TestParseSSEDataJoinsDataLines(t *testing.T) {
	raw := "event: message\n" +
		"data: {\"a\":1,\n" +
		"data: \"b\":2}\n"
	got := parseSSEData(raw)
	want := "{\"a\":1,\n\"b\":2}"
	if got != want {
		t.Fatalf("parseSSEData() = %q, want %q", got, want)
	}
}

func TestSafeFilenamePartFallback(t *testing.T) {
	if got := safeFilenamePart("../"); got != "media" {
		t.Fatalf("safeFilenamePart fallback = %q", got)
	}
}
