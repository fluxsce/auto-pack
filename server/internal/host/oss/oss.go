// Package oss 按报文里的连接信息上传对象并签发临时访问地址。
package oss

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
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

// Probe 用 HeadBucket 校验 endpoint / bucket / 密钥（不写对象）。
func Probe(ctx context.Context, cfg Config) error {
	return probeWithClient(ctx, &http.Client{Timeout: 15 * time.Second}, cfg)
}

func probeWithClient(ctx context.Context, client *http.Client, cfg Config) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := newHead(cfg, time.Now().UTC())
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("oss bucket not found: %s", msg)
	case http.StatusForbidden:
		return fmt.Errorf("oss access denied: %s", msg)
	default:
		return fmt.Errorf("oss probe %s: %s", resp.Status, msg)
	}
}

// Exists 判断对象是否已在桶里。404 视为不存在。
func Exists(cfg Config, objectKey string) (bool, error) {
	return existsWithClient(&http.Client{Timeout: 30 * time.Second}, cfg, objectKey)
}

func existsWithClient(client *http.Client, cfg Config, objectKey string) (bool, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := newHeadObject(cfg, objectKey, time.Now().UTC())
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode/100 == 2 {
		return true, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return false, fmt.Errorf("oss head %s: %s", resp.Status, strings.TrimSpace(string(body)))
}

// Copy 把 srcKey 复制到 destKey，不下载对象内容。
func Copy(cfg Config, srcKey, destKey string) error {
	return copyWithClient(&http.Client{Timeout: 30 * time.Minute}, cfg, srcKey, destKey)
}

func copyWithClient(client *http.Client, cfg Config, srcKey, destKey string) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	req, err := newCopy(cfg, srcKey, destKey, time.Now().UTC())
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("oss copy %s: %s", resp.Status, strings.TrimSpace(string(body)))
}

// Get 读取对象内容。对象不存在时返回错误。
func Get(cfg Config, objectKey string) ([]byte, error) {
	objectKey = strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	req, err := newGet(cfg, objectKey, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("oss get 404: %s", strings.TrimSpace(string(body)))
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("oss get %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// PutBytes 把内存内容上传到 objectKey。
func PutBytes(cfg Config, objectKey string, data []byte) error {
	req, err := newPut(cfg, strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/"), bytes.NewReader(data), int64(len(data)), time.Now().UTC())
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
	return signGetAt(cfg, objectKey, time.Now().Add(time.Duration(sec)*time.Second), "")
}

// SignGetMode 签发 GET 临时地址。mode 为 inline 时浏览器尽量直接打开，attachment 时下载。空 mode 与 SignGet 相同。
func SignGetMode(cfg Config, objectKey string, expire time.Duration, mode string) (string, error) {
	sec := int64(expire / time.Second)
	if sec < 1 {
		sec = 1
	}
	return signGetAt(cfg, objectKey, time.Now().Add(time.Duration(sec)*time.Second), mode)
}

func signGetAt(cfg Config, objectKey string, exp time.Time, mode string) (string, error) {
	host, https, err := endpointHost(cfg)
	if err != nil {
		return "", err
	}
	expires := fmt.Sprintf("%d", exp.Unix())
	resource := "/" + cfg.Bucket + "/" + objectKey
	q := url.Values{}
	q.Set("OSSAccessKeyId", cfg.AccessKeyID)
	q.Set("Expires", expires)
	subs := map[string]string{}
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		subs["security-token"] = token
		q.Set("security-token", token)
	}
	switch mode {
	case "inline", "attachment":
		name := strings.ReplaceAll(pathBase(objectKey), `"`, "")
		subs["response-content-disposition"] = mode + `; filename="` + name + `"`
		q.Set("response-content-disposition", subs["response-content-disposition"])
	}
	if len(subs) > 0 {
		keys := make([]string, 0, len(subs))
		for key := range subs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		resource += "?"
		for i, key := range keys {
			if i > 0 {
				resource += "&"
			}
			resource += key + "=" + url.QueryEscape(subs[key])
		}
	}
	sig := sign(cfg.AccessKeySecret, "GET\n\n\n"+expires+"\n"+resource)
	q.Set("Signature", sig)
	scheme := "https"
	if !https {
		scheme = "http"
	}
	return scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(objectKey) + "?" + q.Encode(), nil
}

func pathBase(key string) string {
	key = strings.TrimRight(strings.ReplaceAll(key, "\\", "/"), "/")
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	if key == "" {
		return "file"
	}
	return key
}

func newHead(cfg Config, now time.Time) (*http.Request, error) {
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	raw := scheme + "://" + cfg.Bucket + "." + host + "/"
	req, err := http.NewRequest(http.MethodHead, raw, nil)
	if err != nil {
		return nil, err
	}
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	resource := "/" + cfg.Bucket + "/"
	canon := ""
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		canon = "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "HEAD\n\n\n" + date + "\n" + canon + resource
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
}

func newHeadObject(cfg Config, objectKey string, now time.Time) (*http.Request, error) {
	objectKey = strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	raw := scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(objectKey)
	req, err := http.NewRequest(http.MethodHead, raw, nil)
	if err != nil {
		return nil, err
	}
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	resource := "/" + cfg.Bucket + "/" + objectKey
	canon := ""
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		canon = "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "HEAD\n\n\n" + date + "\n" + canon + resource
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
}

func newCopy(cfg Config, srcKey, destKey string, now time.Time) (*http.Request, error) {
	srcKey = strings.TrimLeft(strings.ReplaceAll(srcKey, "\\", "/"), "/")
	destKey = strings.TrimLeft(strings.ReplaceAll(destKey, "\\", "/"), "/")
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	raw := scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(destKey)
	req, err := http.NewRequest(http.MethodPut, raw, nil)
	if err != nil {
		return nil, err
	}
	source := "/" + cfg.Bucket + "/" + escapeKey(srcKey)
	req.Header.Set("x-oss-copy-source", source)
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	resource := "/" + cfg.Bucket + "/" + destKey
	canon := "x-oss-copy-source:" + source + "\n"
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		canon += "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "PUT\n\n\n" + date + "\n" + canon + resource
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
}

func newGet(cfg Config, objectKey string, now time.Time) (*http.Request, error) {
	objectKey = strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	raw := scheme + "://" + cfg.Bucket + "." + host + "/" + escapeKey(objectKey)
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	resource := "/" + cfg.Bucket + "/" + objectKey
	canon := ""
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		canon = "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "GET\n\n\n" + date + "\n" + canon + resource
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
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
