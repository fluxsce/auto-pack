package env

import (
	"context"
	"errors"
	"strings"
	"time"

	hostoss "auto-pack/server/internal/host/oss"
	"auto-pack/server/internal/iam"
)

var errOSSKey = errors.New("invalid object key")

// OSSObject 是管理页上的一个目录或文件。
type OSSObject struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Dir      bool   `json:"dir"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

// OSSListResult 是一次浏览。
type OSSListResult struct {
	OK         bool        `json:"ok"`
	Message    string      `json:"message,omitempty"`
	Bucket     string      `json:"bucket,omitempty"`
	Prefix     string      `json:"prefix"`
	NextMarker string      `json:"next_marker,omitempty"`
	Truncated  bool        `json:"truncated"`
	Items      []OSSObject `json:"items"`
}

// OSSSignResult 是一条临时访问地址。
type OSSSignResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
	URL       string `json:"url,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Hours     int    `json:"hours,omitempty"`
}

// OSSList 列出该 OSS 环境桶内某一层目录。只读。
func (s *Service) OSSList(ctx context.Context, actor iam.Actor, recID, prefix, marker string) (OSSListResult, error) {
	cfg, err := s.ossConfigFor(ctx, actor, recID)
	if err != nil {
		return OSSListResult{}, err
	}
	prefix, err = cleanOSSKey(prefix, true)
	if err != nil {
		return OSSListResult{OK: false, Message: err.Error()}, nil
	}
	marker = strings.TrimSpace(marker)
	page, err := hostoss.List(ctx, cfg, prefix, marker)
	if err != nil {
		return OSSListResult{OK: false, Message: err.Error(), Bucket: cfg.Bucket, Prefix: prefix}, nil
	}
	items := make([]OSSObject, 0, len(page.Entries))
	for _, entry := range page.Entries {
		items = append(items, OSSObject{
			Name: entry.Name, Key: entry.Key, Dir: entry.Dir, Size: entry.Size, Modified: entry.LastModified,
		})
	}
	return OSSListResult{
		OK: true, Bucket: cfg.Bucket, Prefix: prefix, NextMarker: page.NextMarker, Truncated: page.Truncated, Items: items,
	}, nil
}

// OSSSign 为桶内对象签发临时 GET 地址。mode 为 view、download 或 link。
func (s *Service) OSSSign(ctx context.Context, actor iam.Actor, recID, key, mode string) (OSSSignResult, error) {
	cfg, raw, err := s.ossConfigRaw(ctx, actor, recID)
	if err != nil {
		return OSSSignResult{}, err
	}
	key, err = cleanOSSKey(key, false)
	if err != nil || key == "" || strings.HasSuffix(key, "/") {
		return OSSSignResult{OK: false, Message: "object key required"}, nil
	}
	hours := hostoss.ExpireHours(raw)
	expire := time.Duration(hours) * time.Hour
	signMode := ""
	switch mode {
	case "view":
		signMode = "inline"
	case "download":
		signMode = "attachment"
	case "link", "":
		signMode = ""
	default:
		return OSSSignResult{OK: false, Message: "mode must be view, download or link"}, nil
	}
	url, err := hostoss.SignGetMode(cfg, key, expire, signMode)
	if err != nil {
		return OSSSignResult{OK: false, Message: err.Error()}, nil
	}
	until := time.Now().Add(expire).In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	return OSSSignResult{OK: true, URL: url, ExpiresAt: until, Hours: hours}, nil
}

func (s *Service) ossConfigFor(ctx context.Context, actor iam.Actor, recID string) (hostoss.Config, error) {
	cfg, _, err := s.ossConfigRaw(ctx, actor, recID)
	return cfg, err
}

func (s *Service) ossConfigRaw(ctx context.Context, actor iam.Actor, recID string) (hostoss.Config, map[string]any, error) {
	row, raw, err := s.DecryptRow(ctx, recID)
	if err != nil {
		return hostoss.Config{}, nil, err
	}
	if err := s.authorizeRead(ctx, actor, row.WorkspaceRecID); err != nil {
		return hostoss.Config{}, nil, err
	}
	kind, err := NormalizeKind(row.EnvKind)
	if err != nil || kind != "oss" {
		return hostoss.Config{}, nil, errors.New("environment must be oss")
	}
	cfg := hostoss.MapConfig(raw)
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return hostoss.Config{}, nil, errors.New("oss env is not configured")
	}
	return cfg, raw, nil
}

func cleanOSSKey(key string, allowEmpty bool) (string, error) {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimLeft(key, "/")
	if key == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errOSSKey
	}
	for _, part := range strings.Split(key, "/") {
		if part == ".." {
			return "", errOSSKey
		}
	}
	return key, nil
}
