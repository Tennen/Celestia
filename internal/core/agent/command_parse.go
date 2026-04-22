package agent

import (
	"strconv"
	"strings"
)

type parsedFlags struct {
	Positionals []string
	Flags       map[string]string
	Bools       map[string]bool
}

func shellFields(input string) []string {
	tokens := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	for _, ch := range input {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func parseFlags(tokens []string) parsedFlags {
	out := parsedFlags{Flags: map[string]string{}, Bools: map[string]bool{}}
	for idx := 0; idx < len(tokens); idx++ {
		token := strings.TrimSpace(tokens[idx])
		if !strings.HasPrefix(token, "--") {
			out.Positionals = append(out.Positionals, token)
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(token, "--"))
		if body == "" {
			continue
		}
		if key, value, ok := strings.Cut(body, "="); ok {
			key = strings.ToLower(strings.TrimSpace(key))
			if key != "" {
				out.Flags[key] = strings.TrimSpace(value)
			}
			continue
		}
		key := strings.ToLower(body)
		next := ""
		if idx+1 < len(tokens) && !strings.HasPrefix(tokens[idx+1], "--") {
			next = tokens[idx+1]
			idx++
		}
		if next == "" {
			out.Bools[key] = true
		} else {
			out.Flags[key] = next
		}
	}
	return out
}

func flagString(flags parsedFlags, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(flags.Flags[strings.ToLower(key)]); value != "" {
			return value
		}
	}
	return ""
}

func flagBool(flags parsedFlags, key string) (bool, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if value, ok := flags.Flags[key]; ok {
		return parseBoolText(value), true
	}
	if value, ok := flags.Bools[key]; ok {
		return value, true
	}
	return false, false
}

func flagFloat(flags parsedFlags, key string) (float64, bool) {
	raw := strings.TrimSpace(flags.Flags[strings.ToLower(key)])
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	return value, err == nil
}

func parseBoolText(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}
