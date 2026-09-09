package release

import "auto-pack/server/internal/release/record"

// PostOptions is stored on AP_WORKSPACE_RELEASE for multi-user auto-release UI restore.
type PostOptions = record.PostOptions

// ParsePostOptionsJSON decodes persisted post options JSON.
func ParsePostOptionsJSON(raw string) (*PostOptions, error) {
	return record.ParsePostOptionsJSON(raw)
}

// MarshalPostOptions encodes post options for AP_WORKSPACE_RELEASE.
func MarshalPostOptions(opts *PostOptions) (string, error) {
	return record.MarshalPostOptions(opts)
}
