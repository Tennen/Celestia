package app

import "testing"

func TestParseRegionServerList(t *testing.T) {
	t.Run("map with list", func(t *testing.T) {
		value := map[string]any{
			"list": []any{
				map[string]any{"id": "US"},
			},
			"pref": "CN",
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from map payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("bare list", func(t *testing.T) {
		value := []any{
			map[string]any{"id": "EU"},
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from bare array payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("invalid shape", func(t *testing.T) {
		if _, ok := parseRegionServerList(map[string]any{"pref": "CN"}); ok {
			t.Fatal("expected invalid payload to fail parsing")
		}
	})
}
