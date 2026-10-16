// Package mns 对接阿里云 MNS HTTP（签名、发/收/删消息）。
// 无 STS 时不签 security-token 行。业务 JSON 放在 MessageBody（Base64）。
package mns

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiVersion   = "2015-06-06"
	headerDate   = "Date"
	headerAuth   = "Authorization"
	headerMD5    = "Content-MD5"
	headerType   = "Content-Type"
	headerMNSVer = "x-mns-version"
	contentXML   = "text/xml"
)

// Config 是平台 mq 环境里的 MNS 连接。
type Config struct {
	Endpoint        string
	RequestQueue    string // 消费：BOMS 打包请求
	ResultQueue     string // 生产：打包结果回执
	AccessKeyID     string
	AccessKeySecret string
	BatchSize       int // 收消息条数，默认 10
}

// Message 是一次拉取到的队列消息（Body 已是 JSON 原文）。
type Message struct {
	MessageID     string
	ReceiptHandle string
	Body          []byte
}

// ErrMessageNotExist 表示队列当前无消息（HTTP 404 + MessageNotExist），属正常空结果。
var ErrMessageNotExist = fmt.Errorf("mns: MessageNotExist")

// MapConfig 读取已解密的 mq 环境。
func MapConfig(m map[string]any) Config {
	batch := 10
	if n, ok := mapInt(m, "batch_size"); ok && n > 0 {
		batch = n
	}
	return Config{
		Endpoint:        mapString(m, "endpoint"),
		RequestQueue:    mapString(m, "request_queue"),
		ResultQueue:     mapString(m, "result_queue"),
		AccessKeyID:     mapString(m, "access_key_id", "access_key"),
		AccessKeySecret: mapString(m, "access_key_secret", "secret_key"),
		BatchSize:       batch,
	}
}

// Client 调用 MNS REST。
type Client struct {
	cfg    Config
	http   *http.Client
	base   string // scheme://host，无尾斜杠
	nowUTC func() time.Time
}

// New 校验配置并构造客户端。
func New(cfg Config) (*Client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint required")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("endpoint invalid")
	}
	base := strings.TrimRight(u.Scheme+"://"+u.Host, "/")
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, fmt.Errorf("access_key_id / access_key_secret required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: 60 * time.Second},
		base:   base,
		nowUTC: func() time.Time { return time.Now().UTC() },
	}, nil
}

// Send 把 JSON 正文写入指定队列（先 Base64 再包 XML）。
func (c *Client) Send(ctx context.Context, queue string, jsonBody []byte) (messageID string, err error) {
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return "", fmt.Errorf("queue required")
	}
	xmlBody := buildSendXML(jsonBody)
	path := "/queues/" + queue + "/messages"
	resp, body, err := c.do(ctx, http.MethodPost, path, xmlBody, contentXML)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", apiError(resp.StatusCode, body)
	}
	var out sendResponse
	if err := xml.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.MessageID), nil
}

// Receive 拉取最多 n 条（进入不可见窗口）；无消息时返回 ErrMessageNotExist。
func (c *Client) Receive(ctx context.Context, queue string, n, waitSeconds int) ([]Message, error) {
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return nil, fmt.Errorf("queue required")
	}
	if n <= 0 {
		n = c.cfg.BatchSize
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}
	path := fmt.Sprintf("/queues/%s/messages?numofmessages=%d&waitseconds=%d", queue, n, waitSeconds)
	return c.getMessages(ctx, path)
}

// Peek 查看最多 n 条，不改变可见性、不返回可删的 ReceiptHandle。
func (c *Client) Peek(ctx context.Context, queue string, n int) ([]Message, error) {
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return nil, fmt.Errorf("queue required")
	}
	if n <= 0 {
		n = c.cfg.BatchSize
	}
	path := fmt.Sprintf("/queues/%s/messages?peekonly=true&numofmessages=%d", queue, n)
	return c.getMessages(ctx, path)
}

func (c *Client) getMessages(ctx context.Context, path string) ([]Message, error) {
	resp, body, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && isMessageNotExist(body) {
		return nil, ErrMessageNotExist
	}
	if resp.StatusCode/100 != 2 {
		return nil, apiError(resp.StatusCode, body)
	}
	return parseReceive(body)
}

// Delete 删除已收下的消息。
func (c *Client) Delete(ctx context.Context, queue, receiptHandle string) error {
	queue = strings.TrimSpace(queue)
	handle := strings.TrimSpace(receiptHandle)
	if queue == "" || handle == "" {
		return fmt.Errorf("queue and ReceiptHandle required")
	}
	path := "/queues/" + queue + "/messages?ReceiptHandle=" + javaURLEncode(handle)
	resp, body, err := c.do(ctx, http.MethodDelete, path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return apiError(resp.StatusCode, body)
	}
	return nil
}

// Probe 验证发结果队列 + 收请求队列（空队列的 MessageNotExist 算成功）。
// 不在结果队列上收消息，避免误领 BOMS 待消费的回执。
func Probe(ctx context.Context, cfg Config) error {
	if strings.TrimSpace(cfg.RequestQueue) == "" {
		return fmt.Errorf("request_queue required")
	}
	if strings.TrimSpace(cfg.ResultQueue) == "" {
		return fmt.Errorf("result_queue required")
	}
	c, err := New(cfg)
	if err != nil {
		return err
	}
	probe := []byte(`{"autoPackEnvProbe":true,"ts":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`)
	if _, err := c.Send(ctx, cfg.ResultQueue, probe); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	_, err = c.Receive(ctx, cfg.RequestQueue, 1, 0)
	if err == nil || err == ErrMessageNotExist {
		return nil
	}
	return fmt.Errorf("receive: %w", err)
}

func (c *Client) do(ctx context.Context, method, resourcePath string, body []byte, contentType string) (*http.Response, []byte, error) {
	date := c.nowUTC().Format(http.TimeFormat)
	md5b64 := ""
	if len(body) > 0 {
		sum := md5.Sum(body)
		md5b64 = base64.StdEncoding.EncodeToString(sum[:])
	}
	stringToSign := signString(method, md5b64, contentType, date, resourcePath)
	sig := sign(c.cfg.AccessKeySecret, stringToSign)

	req, err := http.NewRequestWithContext(ctx, method, c.base+resourcePath, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set(headerDate, date)
	req.Header.Set(headerMNSVer, apiVersion)
	req.Header.Set(headerAuth, "MNS "+c.cfg.AccessKeyID+":"+sig)
	if contentType != "" {
		req.Header.Set(headerType, contentType)
	}
	if md5b64 != "" {
		req.Header.Set(headerMD5, md5b64)
	}
	if len(body) > 0 {
		req.ContentLength = int64(len(body))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		_ = resp.Body.Close()
		return nil, nil, err
	}
	// 重置 Body 供调用方 Close；内容已读完。
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, raw, nil
}

func signString(method, contentMD5, contentType, date, resourcePath string) string {
	var b strings.Builder
	b.WriteString(method)
	b.WriteByte('\n')
	b.WriteString(contentMD5)
	b.WriteByte('\n')
	b.WriteString(contentType)
	b.WriteByte('\n')
	b.WriteString(date)
	b.WriteByte('\n')
	b.WriteString(headerMNSVer)
	b.WriteByte(':')
	b.WriteString(apiVersion)
	b.WriteByte('\n')
	b.WriteString(resourcePath)
	return b.String()
}

func sign(secret, stringToSign string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildSendXML(jsonBody []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(jsonBody)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<Message xmlns="http://mns.aliyuncs.com/doc/v1">`)
	b.WriteString(`<MessageBody>`)
	b.WriteString(encoded)
	b.WriteString(`</MessageBody></Message>`)
	return []byte(b.String())
}

// javaURLEncode 对齐 Java URLEncoder.encode(s, "UTF-8")：空格为 +，保留 * - _ .。
func javaURLEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '*':
			b.WriteByte(c)
		case c == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

type sendResponse struct {
	XMLName   xml.Name `xml:"Message"`
	MessageID string   `xml:"MessageId"`
}

type errorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
	ReqID   string   `xml:"RequestId"`
}

type receiveResponse struct {
	XMLName  xml.Name         `xml:"Messages"`
	Messages []receiveMessage `xml:"Message"`
}

type receiveOne struct {
	XMLName xml.Name `xml:"Message"`
	receiveMessage
}

type receiveMessage struct {
	MessageID     string `xml:"MessageId"`
	MessageBody   string `xml:"MessageBody"`
	ReceiptHandle string `xml:"ReceiptHandle"`
}

func parseReceive(raw []byte) ([]Message, error) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Contains(trimmed, []byte("<Messages")) {
		var multi receiveResponse
		if err := xml.Unmarshal(trimmed, &multi); err != nil {
			return nil, err
		}
		return decodeMessages(multi.Messages)
	}
	var one receiveOne
	if err := xml.Unmarshal(trimmed, &one); err != nil {
		return nil, err
	}
	return decodeMessages([]receiveMessage{one.receiveMessage})
}

func decodeMessages(in []receiveMessage) ([]Message, error) {
	out := make([]Message, 0, len(in))
	for _, m := range in {
		body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.MessageBody))
		if err != nil {
			return nil, fmt.Errorf("MessageBody base64: %w", err)
		}
		out = append(out, Message{
			MessageID:     strings.TrimSpace(m.MessageID),
			ReceiptHandle: strings.TrimSpace(m.ReceiptHandle),
			Body:          body,
		})
	}
	return out, nil
}

func isMessageNotExist(body []byte) bool {
	var er errorResponse
	if xml.Unmarshal(body, &er) != nil {
		return bytes.Contains(body, []byte("MessageNotExist"))
	}
	return er.Code == "MessageNotExist"
}

func apiError(status int, body []byte) error {
	var er errorResponse
	if xml.Unmarshal(body, &er) == nil && er.Code != "" {
		msg := er.Message
		if er.ReqID != "" {
			msg = msg + " requestId=" + er.ReqID
		}
		return fmt.Errorf("mns %d %s: %s", status, er.Code, msg)
	}
	return fmt.Errorf("mns %d: %s", status, strings.TrimSpace(string(body)))
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
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s
			}
		case float64:
			if t == float64(int64(t)) {
				return strconv.FormatInt(int64(t), 10)
			}
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.Itoa(t)
		case int64:
			return strconv.FormatInt(t, 10)
		default:
			if s := strings.TrimSpace(fmt.Sprint(t)); s != "" {
				return s
			}
		}
	}
	return ""
}

func mapInt(m map[string]any, key string) (int, bool) {
	s := mapString(m, key)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
