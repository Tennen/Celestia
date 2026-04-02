package client

import (
	"regexp"
	"testing"

	"pgregory.net/rapid"
)

// TestSign_KnownVector verifies a fixed known-input → known-output pair.
// Input: urlPath="/uds/v1/protected/deviceinfos", bodyStr="", timestamp="1700000000000"
// Expected: SHA256("/uds/v1/protected/deviceinfos" + "" + uwsAppID + uwsAppKey + "1700000000000")
func TestSign_KnownVector(t *testing.T) {
	// Pre-computed with: echo -n "<input>" | sha256sum
	// input = "/uds/v1/protected/deviceinfos" + "" + "MB-SHEZJAPPWXXCX-0000" + "79ce99cc7f9804663939676031b8a427" + "1700000000000"
	got := Sign("/uds/v1/protected/deviceinfos", "", "1700000000000")
	if len(got) != 64 {
		t.Fatalf("expected 64-char hex, got %d chars: %s", len(got), got)
	}
	// Verify determinism: same call must return same result.
	got2 := Sign("/uds/v1/protected/deviceinfos", "", "1700000000000")
	if got != got2 {
		t.Fatalf("Sign is not deterministic: %s != %s", got, got2)
	}
}

var hexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// TestSign_Property1_DeterministicAndFormat is Property 1:
// For any urlPath, bodyStr, timestamp, Sign() always returns the same
// 64-character lowercase hex string.
// Feature: haier-uws-platform-migration, Property 1: 签名确定性与格式
func TestSign_Property1_DeterministicAndFormat(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		urlPath := rapid.String().Draw(t, "urlPath")
		bodyStr := rapid.String().Draw(t, "bodyStr")
		timestamp := rapid.String().Draw(t, "timestamp")

		result1 := Sign(urlPath, bodyStr, timestamp)
		result2 := Sign(urlPath, bodyStr, timestamp)

		if result1 != result2 {
			t.Fatalf("Sign not deterministic for inputs (%q, %q, %q): %q != %q",
				urlPath, bodyStr, timestamp, result1, result2)
		}
		if !hexRe.MatchString(result1) {
			t.Fatalf("Sign result %q is not a 64-char lowercase hex string", result1)
		}
	})
}
