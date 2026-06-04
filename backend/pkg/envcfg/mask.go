package envcfg

import (
	"encoding/json"
	"strings"
)

// MaskConfigJSON redacts sensitive fields before API responses.
func MaskConfigJSON(configJSON string) string {
	raw := strings.TrimSpace(configJSON)
	if raw == "" {
		return "{}"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	for _, key := range []string{
		"password",
		"private_key",
		"private_key_path",
		"auth_password",
		"auth_token",
		"private_key_passphrase",
	} {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			m[key] = "******"
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return string(out)
}
