package env

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"auto-pack/server/internal/crypt"
)

// Field types rendered by the console editor.
const (
	FieldText     = "text"
	FieldPassword = "password"
	FieldNumber   = "number"
	FieldBool     = "bool"
	FieldSelect   = "select"
)

// Categories group kinds in the console.
const (
	CategoryRemote       = "remote"
	CategoryCredential   = "credential"
	CategoryToolchain    = "toolchain"
	CategoryNotification = "notification"
)

// FieldSpec describes one config key for a kind.
type FieldSpec struct {
	Key      string            `json:"key"`
	Type     string            `json:"type"`
	Required bool              `json:"required,omitempty"`
	Secret   bool              `json:"secret,omitempty"`
	Default  any               `json:"default,omitempty"`
	Options  []string          `json:"options,omitempty"`
	Group    string            `json:"group,omitempty"`
	When     map[string]string `json:"when,omitempty"`
}

// KindSpec is one supported environment kind and its editor fields.
type KindSpec struct {
	Kind     string      `json:"kind"`
	Category string      `json:"category"`
	Fields   []FieldSpec `json:"fields"`
}

var kindAliases = map[string]string{
	"nodejs":  "node",
	"node.js": "node",
}

var fieldAliases = map[string]map[string]string{
	"ftp": {
		"base_path":   "remote_dir",
		"remote_path": "remote_dir",
	},
	"sftp": {
		"remote_dir": "remote_path",
		"base_path":  "remote_path",
	},
}

// Catalog is the supported env kinds. HTTP GET /envs/kinds returns this list.
var Catalog = []KindSpec{
	{
		Kind: "ssh", Category: CategoryRemote,
		Fields: []FieldSpec{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: 22},
			{Key: "username", Type: FieldText},
			{Key: "password", Type: FieldPassword, Secret: true},
			{Key: "private_key_path", Type: FieldText},
			{Key: "timeout_sec", Type: FieldNumber, Default: 60},
		},
	},
	{
		Kind: "sftp", Category: CategoryRemote,
		Fields: []FieldSpec{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: 22},
			{Key: "username", Type: FieldText},
			{Key: "password", Type: FieldPassword, Secret: true},
			{Key: "private_key_path", Type: FieldText},
			{Key: "remote_path", Type: FieldText, Default: "/"},
			{Key: "timeout_sec", Type: FieldNumber, Default: 60},
		},
	},
	{
		Kind: "oss", Category: CategoryRemote,
		Fields: []FieldSpec{
			{Key: "endpoint", Type: FieldText, Required: true},
			{Key: "bucket", Type: FieldText, Required: true},
			{Key: "access_key_id", Type: FieldText, Required: true},
			{Key: "access_key_secret", Type: FieldPassword, Secret: true, Required: true},
			{Key: "security_token", Type: FieldPassword, Secret: true},
			{Key: "expire_hours", Type: FieldNumber, Default: 24},
		},
	},
	{
		Kind: "ftp", Category: CategoryRemote,
		Fields: []FieldSpec{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: 21},
			{Key: "username", Type: FieldText},
			{Key: "password", Type: FieldPassword, Secret: true},
			{Key: "remote_dir", Type: FieldText, Default: "/"},
			{Key: "passive", Type: FieldBool, Default: true},
			{Key: "timeout_sec", Type: FieldNumber, Default: 60},
		},
	},
	{
		Kind: "git", Category: CategoryCredential,
		Fields: []FieldSpec{
			{Key: "auth_type", Type: FieldSelect, Default: "https", Options: []string{"none", "https", "ssh"}, Group: "auth"},
			{Key: "auth_token", Type: FieldPassword, Secret: true, Group: "auth", When: map[string]string{"auth_type": "https"}},
			{Key: "auth_username", Type: FieldText, Group: "auth", When: map[string]string{"auth_type": "https"}},
			{Key: "auth_password", Type: FieldPassword, Secret: true, Group: "auth", When: map[string]string{"auth_type": "https"}},
			{Key: "private_key_path", Type: FieldText, Group: "auth", When: map[string]string{"auth_type": "ssh"}},
			{Key: "private_key_passphrase", Type: FieldPassword, Secret: true, Group: "auth", When: map[string]string{"auth_type": "ssh"}},
			{Key: "git_path", Type: FieldText, Group: "runtime"},
			{Key: "username", Type: FieldText, Group: "commit"},
			{Key: "email", Type: FieldText, Group: "commit"},
		},
	},
	{
		Kind: "maven", Category: CategoryToolchain,
		Fields: []FieldSpec{
			{Key: "maven_home", Type: FieldText},
			{Key: "mvn_path", Type: FieldText},
			{Key: "settings_xml", Type: FieldText},
			{Key: "java_home", Type: FieldText},
		},
	},
	{
		Kind: "node", Category: CategoryToolchain,
		Fields: []FieldSpec{
			{Key: "node_path", Type: FieldText},
			{Key: "npm_path", Type: FieldText},
			{Key: "npx_path", Type: FieldText},
			{Key: "npmrc_path", Type: FieldText},
			{Key: "registry", Type: FieldText},
			{Key: "auth_token", Type: FieldPassword, Secret: true},
			{Key: "auth_username", Type: FieldText},
			{Key: "auth_password", Type: FieldPassword, Secret: true},
			{Key: "pnpm_allow_all_builds", Type: FieldBool, Default: true},
		},
	},
	{
		Kind: "jdk", Category: CategoryToolchain,
		Fields: []FieldSpec{
			{Key: "java_home", Type: FieldText},
			{Key: "java_path", Type: FieldText},
		},
	},
	{
		Kind: "mq", Category: CategoryRemote,
		Fields: []FieldSpec{
			{Key: "endpoint", Type: FieldText, Required: true, Group: "mns"},
			{Key: "access_key_id", Type: FieldText, Required: true, Group: "mns"},
			{Key: "access_key_secret", Type: FieldPassword, Secret: true, Required: true, Group: "mns"},
			{Key: "request_queue", Type: FieldText, Required: true, Group: "queues"},
			{Key: "result_queue", Type: FieldText, Required: true, Group: "queues"},
			{Key: "batch_size", Type: FieldNumber, Default: 10, Group: "queues"},
			{Key: "receipt_retries", Type: FieldNumber, Default: 3, Group: "queues"},
		},
	},
	{
		Kind: "smtp", Category: CategoryNotification,
		Fields: []FieldSpec{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: 587},
			{Key: "from", Type: FieldText},
			{Key: "from_name", Type: FieldText, Default: "Auto Pack"},
			{Key: "to", Type: FieldText},
			{Key: "username", Type: FieldText},
			{Key: "password", Type: FieldPassword, Secret: true},
			{Key: "tls", Type: FieldBool, Default: false},
			{Key: "starttls", Type: FieldBool, Default: true},
			{Key: "timeout_sec", Type: FieldNumber, Default: 60},
		},
	},
}

// NormalizeKind maps aliases (nodejs → node) and rejects unknown kinds.
func NormalizeKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	if alias, ok := kindAliases[k]; ok {
		k = alias
	}
	if _, ok := Lookup(k); !ok {
		return "", fmt.Errorf("unknown env_kind %q", kind)
	}
	return k, nil
}

// Lookup returns the spec for a canonical kind.
func Lookup(kind string) (KindSpec, bool) {
	k := strings.ToLower(strings.TrimSpace(kind))
	if alias, ok := kindAliases[k]; ok {
		k = alias
	}
	for _, spec := range Catalog {
		if spec.Kind == k {
			return spec, true
		}
	}
	return KindSpec{}, false
}

// KindCodes returns stored env_kind values that match kind (including aliases).
func KindCodes(kind string) []string {
	k, err := NormalizeKind(kind)
	if err != nil {
		return nil
	}
	if k == "node" {
		return []string{"node", "nodejs"}
	}
	return []string{k}
}

// NormalizeConfig keeps known keys, applies aliases/defaults, and coerces types.
func NormalizeConfig(kind string, cfg map[string]any) (map[string]any, error) {
	spec, ok := Lookup(kind)
	if !ok {
		return nil, fmt.Errorf("unknown env_kind %q", kind)
	}
	src := map[string]any{}
	for k, v := range cfg {
		src[k] = v
	}
	if aliases, ok := fieldAliases[spec.Kind]; ok {
		for from, to := range aliases {
			if _, exists := src[to]; exists {
				continue
			}
			if v, ok := src[from]; ok {
				src[to] = v
			}
		}
	}
	out := map[string]any{}
	for _, f := range spec.Fields {
		v, exists := src[f.Key]
		if !exists || isEmpty(v) {
			if f.Default != nil {
				out[f.Key] = f.Default
			}
			continue
		}
		switch f.Type {
		case FieldNumber:
			if n := asNumber(v); n != nil {
				out[f.Key] = n
			}
		case FieldBool:
			out[f.Key] = asBool(v)
		default:
			if s := asString(v); s != "" {
				out[f.Key] = s
			} else if f.Default != nil {
				out[f.Key] = f.Default
			}
		}
	}
	return out, nil
}

// ValidateConfig checks required keys after normalize/merge.
func ValidateConfig(kind string, cfg map[string]any) error {
	spec, ok := Lookup(kind)
	if !ok {
		return fmt.Errorf("unknown env_kind %q", kind)
	}
	for _, f := range spec.Fields {
		if !f.Required {
			continue
		}
		if isEmpty(cfg[f.Key]) {
			return fmt.Errorf("%s required", f.Key)
		}
	}
	return nil
}

// MergeSecrets copies stored secret values when the incoming field is empty.
func MergeSecrets(kind string, incoming, existing map[string]any) map[string]any {
	spec, ok := Lookup(kind)
	if !ok {
		return incoming
	}
	out := map[string]any{}
	for k, v := range incoming {
		out[k] = v
	}
	for _, f := range spec.Fields {
		if !f.Secret && f.Type != FieldPassword {
			continue
		}
		if !isEmpty(out[f.Key]) {
			continue
		}
		if v, ok := existing[f.Key]; ok && !isEmpty(v) {
			out[f.Key] = v
		}
	}
	return out
}

// Redact clears secret values for HTTP. SecretsSet lists keys that had a value.
func Redact(kind string, cfg map[string]any) (map[string]any, []string) {
	spec, ok := Lookup(kind)
	if !ok {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	var set []string
	for _, f := range spec.Fields {
		v, ok := cfg[f.Key]
		if !ok {
			continue
		}
		if f.Secret || f.Type == FieldPassword {
			if !isEmpty(v) {
				set = append(set, f.Key)
			}
			continue
		}
		out[f.Key] = v
	}
	return out, set
}

// ConfigHint builds a short mask from public fields.
func ConfigHint(kind string, cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	prefer := []string{"host", "endpoint", "request_queue", "from", "to", "java_home", "maven_home", "node_path", "username", "email", "registry"}
	secret := secretKeys(kind)
	for _, k := range prefer {
		if secret[k] {
			continue
		}
		if s := asString(cfg[k]); s != "" {
			return crypt.Hint(s)
		}
	}
	if spec, ok := Lookup(kind); ok {
		for _, f := range spec.Fields {
			if f.Secret || f.Type == FieldPassword || f.Type == FieldBool {
				continue
			}
			if s := asString(cfg[f.Key]); s != "" {
				return crypt.Hint(s)
			}
		}
	}
	return "configured"
}

func secretKeys(kind string) map[string]bool {
	out := map[string]bool{}
	spec, ok := Lookup(kind)
	if !ok {
		return out
	}
	for _, f := range spec.Fields {
		if f.Secret || f.Type == FieldPassword {
			out[f.Key] = true
		}
	}
	return out
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == ""
	default:
		return false
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	default:
		return false
	}
}

func asNumber(v any) any {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil
		}
		return f
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return f
	default:
		return nil
	}
}
