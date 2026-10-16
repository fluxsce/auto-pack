package env_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"auto-pack/server/internal/env"
)

func TestMQProduceConsumeManual(t *testing.T) {
	var mu sync.Mutex
	type item struct {
		id, handle, body string
		gone             bool
	}
	var items []item
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			raw, _ := io.ReadAll(r.Body)
			body := extractXMLBody(raw)
			mu.Lock()
			id := fmt.Sprintf("m%d", len(items)+1)
			handle := fmt.Sprintf("h%d+/=", len(items)+1)
			items = append(items, item{id, handle, body, false})
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `<Message xmlns="http://mns.aliyuncs.com/doc/v1"><MessageId>%s</MessageId></Message>`, id)
		case http.MethodGet:
			mu.Lock()
			var live []item
			for _, it := range items {
				if !it.gone {
					live = append(live, it)
				}
			}
			mu.Unlock()
			if len(live) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code></Error>`)
				return
			}
			var b strings.Builder
			b.WriteString(`<Messages xmlns="http://mns.aliyuncs.com/doc/v1">`)
			for _, it := range live {
				fmt.Fprintf(&b, `<Message><MessageId>%s</MessageId><ReceiptHandle>%s</ReceiptHandle><MessageBody>%s</MessageBody></Message>`,
					it.id, it.handle, base64.StdEncoding.EncodeToString([]byte(it.body)))
			}
			b.WriteString(`</Messages>`)
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, b.String())
		case http.MethodDelete:
			handle := decodeJavaHandle(r.URL.RawQuery)
			mu.Lock()
			for i := range items {
				if items[i].handle == handle {
					items[i].gone = true
				}
			}
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)

	svc, ctx, actor := setupEnv(t)
	cfg := map[string]any{
		"endpoint":          srv.URL,
		"request_queue":     "REQ",
		"result_queue":      "RES",
		"access_key_id":     "ak",
		"access_key_secret": "sk",
	}
	prod, err := svc.MQProduce(ctx, actor, env.MQTestIn{
		EnvKind: "mq",
		Config:  cfg,
		Queue:   "request",
		Body:    `{"taskId":"T-UI"}`,
	})
	if err != nil || !prod.OK || prod.MessageID == "" {
		t.Fatalf("%+v %v", prod, err)
	}
	cons, err := svc.MQConsume(ctx, actor, env.MQTestIn{
		EnvKind: "mq",
		Config:  cfg,
		Queue:   "request",
		Delete:  true,
	})
	if err != nil || !cons.OK || len(cons.Messages) != 1 || cons.Messages[0].Body != `{"taskId":"T-UI"}` || !cons.Messages[0].Deleted {
		t.Fatalf("%+v %v", cons, err)
	}
	empty, err := svc.MQConsume(ctx, actor, env.MQTestIn{EnvKind: "mq", Config: cfg, Queue: "request"})
	if err != nil || !empty.OK || !empty.Empty {
		t.Fatalf("%+v %v", empty, err)
	}
}

func extractXMLBody(raw []byte) string {
	s := string(raw)
	const tag = "<MessageBody>"
	i := strings.Index(s, tag)
	if i < 0 {
		return ""
	}
	s = s[i+len(tag):]
	j := strings.Index(s, "</MessageBody>")
	if j < 0 {
		return ""
	}
	b, _ := base64.StdEncoding.DecodeString(s[:j])
	return string(b)
}

func decodeJavaHandle(rawQuery string) string {
	const key = "ReceiptHandle="
	i := strings.Index(rawQuery, key)
	if i < 0 {
		return ""
	}
	enc := rawQuery[i+len(key):]
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
