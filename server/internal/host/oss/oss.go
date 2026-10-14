// Package oss 按报文里的连接信息上传对象并签发临时访问地址。
package oss

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config 是平台环境里维护的 OSS 连接信息。
type Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
}

// PutFile 把本地文件上传到 objectKey。
func PutFile(cfg Config, local, objectKey string) error {
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	req, err := newPut(cfg, objectKey, f, st.Size(), time.Now().UTC())
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("oss put %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// SignGet 签发 GET 临时访问地址。expire 至少 1 秒。
func SignGet(cfg Config, objectKey string, expire time.Duration) (string, error) {
	sec := int64(expire / time.Second)
	if sec < 1 {
		sec = 1
	}
	return signGetAt(cfg, objectKey, time.Now().Add(time.Duration(sec)*time.Second))
}

func signGetAt(cfg Config, objectKey string, exp time.Time) (string, error) {
	host, https, err := endpointHost(cfg)
	if err != nil {
		return "", err
	}
	expires := fmt.Sprintf("%d", exp.Unix())
	resource := "/" + cfg.Bucket + "/" + objectKey
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		resource += "?security-token=" + token
	}
	sig := sign(cfg.AccessKeySecret, "GET\n\n\n"+expires+"\n"+resource)
	q := url.Values{}
	q.Set("OSSAccessKeyId", cfg.AccessKeyID)
	q.Set("Expires", expires)
	q.Set("Signature", sig)
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		q.Set("security-token", token)
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	return scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(objectKey) + "?" + q.Encode(), nil
}

func newPut(cfg Config, objectKey string, body io.Reader, size int64, now time.Time) (*http.Request, error) {
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	raw := scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(objectKey)
	req, err := http.NewRequest(http.MethodPut, raw, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	resource := "/" + cfg.Bucket + "/" + objectKey
	canon := ""
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		canon = "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "PUT\n\napplication/octet-stream\n" + date + "\n" + canon + resource
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
}

func sign(secret, stringToSign string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func endpointHost(cfg Config) (string, bool, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.Bucket) == "" ||
		strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return "", false, fmt.Errorf("oss config is incomplete")
	}
	raw := strings.TrimSpace(cfg.Endpoint)
	https := true
	switch {
	case strings.HasPrefix(raw, "https://"):
		raw = strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		raw = strings.TrimPrefix(raw, "http://")
		https = false
	}
	raw = strings.TrimRight(raw, "/")
	if raw == "" || strings.Contains(raw, "/") {
		return "", false, fmt.Errorf("oss endpoint is invalid")
	}
	return raw, https, nil
}

// MapConfig 读取已解密的 oss 环境。
func MapConfig(m map[string]any) Config {
	return Config{
		Endpoint:        mapString(m, "endpoint"),
		Bucket:          mapString(m, "bucket"),
		AccessKeyID:     mapString(m, "access_key_id", "accessKeyId"),
		AccessKeySecret: mapString(m, "access_key_secret", "accessKeySecret"),
		SecurityToken:   mapString(m, "security_token", "securityToken"),
	}
}

// ExpireHours 读取临时链接有效小时数，缺省 24。
func ExpireHours(m map[string]any) int {
	if m == nil {
		return 24
	}
	switch n := m["expire_hours"].(type) {
	case float64:
		if n > 0 {
			return int(n)
		}
	case int:
		if n > 0 {
			return n
		}
	case string:
		var parsed int
		for _, r := range strings.TrimSpace(n) {
			if r < '0' || r > '9' {
				return 24
			}
			parsed = parsed*10 + int(r-'0')
		}
		if parsed > 0 {
			return parsed
		}
	}
	return 24
}

func mapString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			if t := strings.TrimSpace(s); t != "" {
				return t
			}
		}
	}
	return ""
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
