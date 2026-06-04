package envcfg

import (
	"encoding/json"
	"strings"
)

// MergeSecrets keeps stored secrets when the client sends masked placeholders.
func MergeSecrets(oldJSON, newJSON string) (string, error) {
	oldM, err := decodeMap(oldJSON)
	if err != nil {
		return newJSON, err
	}
	newM, err := decodeMap(newJSON)
	if err != nil {
		return newJSON, err
	}
	for _, key := range []string{
		"password",
		"private_key",
		"private_key_path",
		"auth_password",
		"auth_token",
		"private_key_passphrase",
	} {
		if isMaskedValue(newM[key]) {
			if v, ok := oldM[key]; ok {
				newM[key] = v
			} else {
				delete(newM, key)
			}
		}
	}
	out, err := json.Marshal(newM)
	if err != nil {
		return newJSON, err
	}
	return string(out), nil
}

func decodeMap(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func isMaskedValue(v any) bool {
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == "******"
}
