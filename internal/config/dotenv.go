package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ParseDotenv parses a minimal dotenv format: KEY=VALUE lines, '#' comments,
// blank lines, optional "export " prefix and optional single or double quotes
// around the value. Variable expansion is intentionally not supported so that
// generated secrets are taken literally.
func ParseDotenv(r io.Reader) (map[string]string, error) {
	out := make(map[string]string)
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		raw = strings.TrimPrefix(raw, "export ")
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", line)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return nil, fmt.Errorf("line %d: invalid key %q", line, key)
		}
		value = strings.TrimSpace(value)
		if n := len(value); n >= 2 {
			if (value[0] == '"' && value[n-1] == '"') || (value[0] == '\'' && value[n-1] == '\'') {
				value = value[1 : n-1]
			}
		}
		out[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadDotenvFile parses the file at path. A missing file is not an error and
// yields an empty map.
func LoadDotenvFile(path string) (map[string]string, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from operator configuration (MESSORA_SECRETS_FILE)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseDotenv(f)
}

func validEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
