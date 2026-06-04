package envcfg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Parse unmarshals config_json for the given env_type.
func Parse(envType, configJSON string) (any, error) {
	envType = strings.TrimSpace(strings.ToLower(envType))
	raw := strings.TrimSpace(configJSON)
	if raw == "" {
		raw = "{}"
	}
	switch envType {
	case TypeSSH:
		var c SSHConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateSSH(c)
	case TypeSFTP:
		var c SFTPConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateSFTP(c)
	case TypeFTP:
		var c FTPConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateFTP(c)
	case TypeGit:
		var c GitConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateGit(c)
	case TypeMaven:
		var c MavenConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateMaven(c)
	case TypeNodeJS:
		var c NodeJSConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateNodeJS(c)
	case TypeJDK:
		var c JDKConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateJDK(c)
	case TypeSMTP:
		var c SMTPConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		return c, ValidateSMTP(c)
	default:
		return nil, fmt.Errorf("envcfg: unknown env_type %q", envType)
	}
}

// IsValidType reports whether envType is supported.
func IsValidType(envType string) bool {
	envType = strings.TrimSpace(strings.ToLower(envType))
	for _, t := range AllTypes {
		if t == envType {
			return true
		}
	}
	return false
}
