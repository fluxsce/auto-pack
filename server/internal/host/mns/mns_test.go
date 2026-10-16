package mns

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignStringGET(t *testing.T) {
	got := signString("GET", "", "", "Wed, 15 Oct 2025 08:00:00 GMT",
		"/queues/BOMS-DEVTSK-PACK-REQ-TEST/messages?numofmessages=10&waitseconds=0")
	want := "GET\n\n\nWed, 15 Oct 2025 08:00:00 GMT\nx-mns-version:2015-06-06\n/queues/BOMS-DEVTSK-PACK-REQ-TEST/messages?numofmessages=10&waitseconds=0"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestJavaURLEncode(t *testing.T) {
	if got := javaURLEncode("a+b/c= d"); got != "a%2Bb%2Fc%3D+d" {
		t.Fatal(got)
	}
	if got := javaURLEncode("abc-._*"); got != "abc-._*" {
		t.Fatal(got)
	}
}

func TestSendReceiveDelete(t *testing.T) {
	var sawAuth string
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			body, _ := io.ReadAll(r.Body)
			sum := md5.Sum(body)
			wantMD5 := base64.StdEncoding.EncodeToString(sum[:])
			if r.Header.Get("Content-MD5") != wantMD5 {
				t.Fatalf("md5 %s want %s", r.Header.Get("Content-MD5"), wantMD5)
			}
			if !strings.Contains(string(body), "MessageBody") {
				t.Fatalf("body %s", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><Message xmlns="http://mns.aliyuncs.com/doc/v1"><MessageId>mid-1</MessageId></Message>`)
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			raw := base64.StdEncoding.EncodeToString([]byte(`{"taskId":"T1"}`))
			_, _ = io.WriteString(w, `<?xml version="1.0"?><Messages xmlns="http://mns.aliyuncs.com/doc/v1"><Message><MessageId>mid-2</MessageId><ReceiptHandle>rh+/=</ReceiptHandle><MessageBody>`+raw+`</MessageBody></Message></Messages>`)
		case r.Method == http.MethodDelete:
			if !strings.Contains(r.URL.RawQuery, "ReceiptHandle=") {
				t.Fatalf("query %s", r.URL.RawQuery)
			}
			if !strings.Contains(r.URL.RawQuery, "rh%2B%2F%3D") {
				t.Fatalf("encoded handle %s", r.URL.RawQuery)
			}
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("%s %s", r.Method, r.URL)
		}
	}))
	defer srv.Close()

	c, err := New(Config{
		Endpoint:        srv.URL,
		RequestQueue:    "BOMS-DEVTSK-PACK-REQ-TEST",
		ResultQueue:     "BOMS-DEVTSK-PACK-RESULT-TEST",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.nowUTC = func() time.Time { return time.Date(2025, 10, 15, 8, 0, 0, 0, time.UTC) }

	id, err := c.Send(context.Background(), "BOMS-DEVTSK-PACK-RESULT-TEST", []byte(`{"ok":true}`))
	if err != nil || id != "mid-1" {
		t.Fatalf("send id=%s err=%v", id, err)
	}
	if !strings.HasPrefix(sawAuth, "MNS ak:") {
		t.Fatalf("auth %s", sawAuth)
	}

	msgs, err := c.Receive(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", 10, 0)
	if err != nil || len(msgs) != 1 || string(msgs[0].Body) != `{"taskId":"T1"}` {
		t.Fatalf("%+v %v", msgs, err)
	}
	if err := c.Delete(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", msgs[0].ReceiptHandle); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("delete not called")
	}
}

func TestReceiveEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<?xml version="1.0"?><Error><Code>MessageNotExist</Code><Message>no</Message></Error>`)
	}))
	defer srv.Close()
	c, err := New(Config{Endpoint: srv.URL, AccessKeyID: "ak", AccessKeySecret: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Receive(context.Background(), "Q", 10, 0)
	if err != ErrMessageNotExist {
		t.Fatalf("err=%v", err)
	}
}

func TestPeek(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("peekonly") != "true" {
			t.Fatalf("query %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		raw := base64.StdEncoding.EncodeToString([]byte(`{"peek":true}`))
		_, _ = io.WriteString(w, `<?xml version="1.0"?><Messages xmlns="http://mns.aliyuncs.com/doc/v1"><Message><MessageId>mid-p</MessageId><MessageBody>`+raw+`</MessageBody></Message></Messages>`)
	}))
	defer srv.Close()
	c, err := New(Config{Endpoint: srv.URL, AccessKeyID: "ak", AccessKeySecret: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := c.Peek(context.Background(), "Q", 5)
	if err != nil || len(msgs) != 1 || string(msgs[0].Body) != `{"peek":true}` || msgs[0].MessageID != "mid-p" {
		t.Fatalf("%+v %v", msgs, err)
	}
}

func TestMapConfig(t *testing.T) {
	cfg := MapConfig(map[string]any{
		"endpoint":          "https://123.mns.cn-hangzhou.aliyuncs.com",
		"request_queue":     "BOMS-DEVTSK-PACK-REQ-TEST",
		"result_queue":      "BOMS-DEVTSK-PACK-RESULT-TEST",
		"access_key_id":     "ak",
		"access_key_secret": "sk",
		"batch_size":        float64(5),
	})
	if cfg.Endpoint == "" || cfg.RequestQueue == "" || cfg.ResultQueue == "" || cfg.BatchSize != 5 {
		t.Fatalf("%+v", cfg)
	}
}

func TestProbeSendAndReceiveEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `<Message xmlns="http://mns.aliyuncs.com/doc/v1"><MessageId>p1</MessageId></Message>`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code></Error>`)
	}))
	defer srv.Close()
	err := Probe(context.Background(), Config{
		Endpoint:        srv.URL,
		RequestQueue:    "REQ",
		ResultQueue:     "RESULT",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// fakeMNS 内存队列：POST 入队，GET 出队，DELETE 按句柄确认，覆盖生产→消费→删除。
type fakeMNS struct {
	t        *testing.T
	mu       sync.Mutex
	queues   map[string][]fakeMsg
	nextID   int
	posted   int
	received int
	deleted  int
}

type fakeMsg struct {
	id     string
	handle string
	body   string // base64 in XML; we store raw JSON
	gone   bool
}

func newFakeMNS(t *testing.T) (*fakeMNS, *httptest.Server) {
	t.Helper()
	f := &fakeMNS{t: t, queues: map[string][]fakeMsg{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeMNS) serve(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// queues/{name}/messages
	if len(parts) < 3 || parts[0] != "queues" || parts[2] != "messages" {
		f.t.Fatalf("path %s", r.URL.Path)
	}
	queue := parts[1]
	switch r.Method {
	case http.MethodPost:
		raw, _ := io.ReadAll(r.Body)
		body := extractMessageBody(raw)
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			f.t.Fatalf("body b64: %v", err)
		}
		f.mu.Lock()
		f.nextID++
		id := fmt.Sprintf("mid-%d", f.nextID)
		handle := fmt.Sprintf("rh-%d+/=", f.nextID)
		f.queues[queue] = append(f.queues[queue], fakeMsg{id: id, handle: handle, body: string(decoded)})
		f.posted++
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `<Message xmlns="http://mns.aliyuncs.com/doc/v1"><MessageId>%s</MessageId></Message>`, id)
	case http.MethodGet:
		f.mu.Lock()
		var out []fakeMsg
		for i := range f.queues[queue] {
			if !f.queues[queue][i].gone {
				out = append(out, f.queues[queue][i])
				if len(out) >= 10 {
					break
				}
			}
		}
		f.received++
		f.mu.Unlock()
		if len(out) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code><Message>empty</Message></Error>`)
			return
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><Messages xmlns="http://mns.aliyuncs.com/doc/v1">`)
		for _, m := range out {
			enc := base64.StdEncoding.EncodeToString([]byte(m.body))
			fmt.Fprintf(&b, `<Message><MessageId>%s</MessageId><ReceiptHandle>%s</ReceiptHandle><MessageBody>%s</MessageBody></Message>`,
				m.id, m.handle, enc)
		}
		b.WriteString(`</Messages>`)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, b.String())
	case http.MethodDelete:
		handle := decodeJavaQueryValue(r.URL.RawQuery, "ReceiptHandle")
		f.mu.Lock()
		found := false
		for i := range f.queues[queue] {
			if f.queues[queue][i].handle == handle && !f.queues[queue][i].gone {
				f.queues[queue][i].gone = true
				f.deleted++
				found = true
				break
			}
		}
		f.mu.Unlock()
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code></Error>`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		f.t.Fatalf("%s", r.Method)
	}
}

func extractMessageBody(xmlRaw []byte) string {
	const tag = "<MessageBody>"
	s := string(xmlRaw)
	i := strings.Index(s, tag)
	if i < 0 {
		return ""
	}
	s = s[i+len(tag):]
	j := strings.Index(s, "</MessageBody>")
	if j < 0 {
		return ""
	}
	return s[:j]
}

// decodeJavaQueryValue 按 Java URLEncoder 逆解：%XX 还原，+ 为空格。
func decodeJavaQueryValue(rawQuery, key string) string {
	prefix := key + "="
	i := strings.Index(rawQuery, prefix)
	if i < 0 {
		return ""
	}
	enc := rawQuery[i+len(prefix):]
	if amp := strings.IndexByte(enc, '&'); amp >= 0 {
		enc = enc[:amp]
	}
	var b strings.Builder
	for i := 0; i < len(enc); {
		if enc[i] == '%' && i+2 < len(enc) {
			var x byte
			_, _ = fmt.Sscanf(enc[i+1:i+3], "%02X", &x)
			b.WriteByte(x)
			i += 3
			continue
		}
		if enc[i] == '+' {
			b.WriteByte(' ')
			i++
			continue
		}
		b.WriteByte(enc[i])
		i++
	}
	return b.String()
}

func TestProduceConsumeDeleteRoundTrip(t *testing.T) {
	fake, srv := newFakeMNS(t)
	c, err := New(Config{
		Endpoint:        srv.URL,
		RequestQueue:    "BOMS-DEVTSK-PACK-REQ-TEST",
		ResultQueue:     "BOMS-DEVTSK-PACK-RESULT-TEST",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"taskId":"TSK-ROUND","subsystemType":"DATAHUB"}`)
	id, err := c.Send(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", payload)
	if err != nil || id == "" {
		t.Fatalf("send %s %v", id, err)
	}
	msgs, err := c.Receive(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", 10, 0)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("recv %+v %v", msgs, err)
	}
	if string(msgs[0].Body) != string(payload) {
		t.Fatalf("body %s", msgs[0].Body)
	}
	if err := c.Delete(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", msgs[0].ReceiptHandle); err != nil {
		t.Fatal(err)
	}
	_, err = c.Receive(context.Background(), "BOMS-DEVTSK-PACK-REQ-TEST", 10, 0)
	if err != ErrMessageNotExist {
		t.Fatalf("after delete want empty, got %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.posted != 1 || fake.deleted != 1 {
		t.Fatalf("posted=%d deleted=%d", fake.posted, fake.deleted)
	}
}

func TestProduceConsumeResultQueue(t *testing.T) {
	_, srv := newFakeMNS(t)
	c, err := New(Config{Endpoint: srv.URL, AccessKeyID: "ak", AccessKeySecret: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	receipt := []byte(`{"taskId":"T1","resultStatus":"SUCCESS"}`)
	if _, err := c.Send(context.Background(), "BOMS-DEVTSK-PACK-RESULT-TEST", receipt); err != nil {
		t.Fatal(err)
	}
	msgs, err := c.Receive(context.Background(), "BOMS-DEVTSK-PACK-RESULT-TEST", 1, 0)
	if err != nil || len(msgs) != 1 || string(msgs[0].Body) != string(receipt) {
		t.Fatalf("%+v %v", msgs, err)
	}
	if err := c.Delete(context.Background(), "BOMS-DEVTSK-PACK-RESULT-TEST", msgs[0].ReceiptHandle); err != nil {
		t.Fatal(err)
	}
}
