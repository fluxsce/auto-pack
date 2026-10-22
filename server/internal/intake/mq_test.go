package intake

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"path/filepath"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/crypt"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/mns"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func TestOpenMNSRequiresQueues(t *testing.T) {
	_, err := OpenMNS(mns.Config{Endpoint: "https://123.mns.cn-hangzhou.aliyuncs.com"})
	if err != errNoQueue {
		t.Fatal(err)
	}
	_, err = OpenMNS(mns.Config{})
	if err != errNoEndpoint {
		t.Fatal(err)
	}
}

func TestReadMQMap(t *testing.T) {
	raw := map[string]any{
		"endpoint":          "https://123.mns.cn-hangzhou.aliyuncs.com",
		"request_queue":     "BOMS-DEVTSK-PACK-REQ-TEST",
		"result_queue":      "BOMS-DEVTSK-PACK-RESULT-TEST",
		"access_key_id":     "ak",
		"access_key_secret": "sk",
		"receipt_retries":   float64(5),
		"batch_size":        float64(10),
	}
	cfg, retries, ok := readMQMap(raw)
	if !ok || cfg.RequestQueue != "BOMS-DEVTSK-PACK-REQ-TEST" || retries != 5 || cfg.BatchSize != 10 {
		t.Fatalf("%+v retries=%d ok=%v", cfg, retries, ok)
	}
	if _, _, ok := readMQMap(map[string]any{"endpoint": "https://x"}); ok {
		t.Fatal("incomplete")
	}
	if _, _, ok := readMQMap(nil); ok {
		t.Fatal("nil")
	}
}

type memQueue struct {
	mu      sync.Mutex
	items   []memItem
	deleted []string
	posted  [][]byte
}

type memItem struct {
	id, handle string
	body       []byte
	gone       bool
	held       bool
}

func startMemMNS(t *testing.T, reqQ, resQ string) (*memQueue, *httptest.Server) {
	t.Helper()
	q := &memQueue{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			t.Fatalf("path %s", r.URL.Path)
		}
		name := parts[1]
		switch r.Method {
		case http.MethodPost:
			raw, _ := io.ReadAll(r.Body)
			body := decodeXMLBody(raw)
			q.mu.Lock()
			id := fmt.Sprintf("id-%d", len(q.posted)+1)
			handle := fmt.Sprintf("h-%d+/=", len(q.posted)+1)
			q.posted = append(q.posted, append([]byte(nil), body...))
			if name == reqQ {
				q.items = append(q.items, memItem{id: id, handle: handle, body: body})
			}
			q.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `<Message xmlns="http://mns.aliyuncs.com/doc/v1"><MessageId>%s</MessageId></Message>`, id)
		case http.MethodGet:
			if name != reqQ {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code></Error>`)
				return
			}
			q.mu.Lock()
			var live []memItem
			for _, it := range q.items {
				if !it.gone && !it.held {
					live = append(live, it)
				}
			}
			q.mu.Unlock()
			if len(live) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<Error><Code>MessageNotExist</Code></Error>`)
				return
			}
			var b strings.Builder
			b.WriteString(`<Messages xmlns="http://mns.aliyuncs.com/doc/v1">`)
			for _, it := range live {
				fmt.Fprintf(&b, `<Message><MessageId>%s</MessageId><ReceiptHandle>%s</ReceiptHandle><MessageBody>%s</MessageBody></Message>`,
					it.id, it.handle, base64.StdEncoding.EncodeToString(it.body))
			}
			b.WriteString(`</Messages>`)
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, b.String())
		case http.MethodPut:
			handle := decodeHandle(r.URL.RawQuery)
			next := "vis-" + handle
			q.mu.Lock()
			for i := range q.items {
				if q.items[i].handle == handle && !q.items[i].gone {
					q.items[i].handle = next
					q.items[i].held = true
					break
				}
			}
			q.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `<ChangeVisibility><ReceiptHandle>%s</ReceiptHandle><NextVisibleTime>1</NextVisibleTime></ChangeVisibility>`, next)
		case http.MethodDelete:
			handle := decodeHandle(r.URL.RawQuery)
			q.mu.Lock()
			for i := range q.items {
				if q.items[i].handle == handle && !q.items[i].gone {
					q.items[i].gone = true
					q.deleted = append(q.deleted, handle)
					break
				}
			}
			q.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("%s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)
	return q, srv
}

func decodeXMLBody(raw []byte) []byte {
	s := string(raw)
	const tag = "<MessageBody>"
	i := strings.Index(s, tag)
	if i < 0 {
		return nil
	}
	s = s[i+len(tag):]
	j := strings.Index(s, "</MessageBody>")
	if j < 0 {
		return nil
	}
	out, _ := base64.StdEncoding.DecodeString(s[:j])
	return out
}

func decodeHandle(rawQuery string) string {
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

func openTestQueue(t *testing.T, srv *httptest.Server) *MNSQueue {
	t.Helper()
	q, err := OpenMNS(mns.Config{
		Endpoint:        srv.URL,
		RequestQueue:    "BOMS-DEVTSK-PACK-REQ-TEST",
		ResultQueue:     "BOMS-DEVTSK-PACK-RESULT-TEST",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		BatchSize:       10,
	})
	if err != nil {
		t.Fatal(err)
	}
	q.idleWait = 20 * time.Millisecond
	q.errWait = 20 * time.Millisecond
	return q
}

func TestMNSQueuePublishProduce(t *testing.T) {
	mem, srv := startMemMNS(t, "BOMS-DEVTSK-PACK-REQ-TEST", "BOMS-DEVTSK-PACK-RESULT-TEST")
	q := openTestQueue(t, srv)
	body := []byte(`{"taskId":"T-OUT","resultStatus":"SUCCESS"}`)
	if err := q.Publish(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	mem.mu.Lock()
	defer mem.mu.Unlock()
	if len(mem.posted) != 1 || string(mem.posted[0]) != string(body) {
		t.Fatalf("posted=%v", mem.posted)
	}
}

func TestMNSQueueListenConsumeThenDelete(t *testing.T) {
	mem, srv := startMemMNS(t, "BOMS-DEVTSK-PACK-REQ-TEST", "BOMS-DEVTSK-PACK-RESULT-TEST")
	q := openTestQueue(t, srv)
	notice := []byte(`{"taskId":"T-IN","subsystemType":"DATAHUB"}`)
	if _, err := q.client.Send(context.Background(), q.requestQueue, notice); err != nil {
		t.Fatal(err)
	}

	got := make(chan []byte, 1)
	if err := q.Listen(func(_ context.Context, body []byte) error {
		got <- append([]byte(nil), body...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	select {
	case body := <-got:
		if string(body) != string(notice) {
			t.Fatalf("body %s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consume timeout")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mem.mu.Lock()
		n := len(mem.deleted)
		mem.mu.Unlock()
		if n == 1 {
			mem.mu.Lock()
			gotHandle := mem.deleted[0]
			mem.mu.Unlock()
			if !strings.HasPrefix(gotHandle, "vis-") {
				t.Fatalf("deleted with original handle %s", gotHandle)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("message was not deleted after successful handle")
}

func TestMNSQueueListenKeepsMessageOnHandleError(t *testing.T) {
	mem, srv := startMemMNS(t, "BOMS-DEVTSK-PACK-REQ-TEST", "BOMS-DEVTSK-PACK-RESULT-TEST")
	q := openTestQueue(t, srv)
	if _, err := q.client.Send(context.Background(), q.requestQueue, []byte(`{"taskId":"T-FAIL"}`)); err != nil {
		t.Fatal(err)
	}
	handled := make(chan struct{}, 1)
	if err := q.Listen(func(context.Context, []byte) error {
		select {
		case handled <- struct{}{}:
		default:
		}
		return errors.New("business reject")
	}); err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("handle not called")
	}
	time.Sleep(50 * time.Millisecond)
	mem.mu.Lock()
	defer mem.mu.Unlock()
	if len(mem.deleted) != 0 {
		t.Fatalf("must not delete on handle error, deleted=%v", mem.deleted)
	}
}

func TestMNSQueueProduceThenConsumeRoundTrip(t *testing.T) {
	// 结果队列生产 + 请求队列消费删除，覆盖接入层两端逻辑。
	mem, srv := startMemMNS(t, "BOMS-DEVTSK-PACK-REQ-TEST", "BOMS-DEVTSK-PACK-RESULT-TEST")
	q := openTestQueue(t, srv)

	receipt := []byte(`{"taskId":"T-RT","resultStatus":"FAILED","resultMessage":"x"}`)
	if err := q.Publish(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	mem.mu.Lock()
	if len(mem.posted) != 1 {
		mem.mu.Unlock()
		t.Fatal("produce")
	}
	mem.mu.Unlock()

	reqBody := []byte(`{"taskId":"T-RT","customerBranch":"release/x"}`)
	if _, err := q.client.Send(context.Background(), q.requestQueue, reqBody); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	if err := q.Listen(func(_ context.Context, body []byte) error {
		if string(body) != string(reqBody) {
			t.Errorf("consume body %s", body)
		}
		close(done)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mem.mu.Lock()
		ok := len(mem.deleted) == 1
		mem.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("consume ack missing")
}

func TestRunEnvFollowsEnabledMQ(t *testing.T) {
	const reqQ = "BOMS-DEVTSK-PACK-REQ-TEST"
	const resQ = "BOMS-DEVTSK-PACK-RESULT-TEST"
	mem, srv := startMemMNS(t, reqQ, resQ)
	cfg := mns.Config{
		Endpoint:        srv.URL,
		RequestQueue:    reqQ,
		ResultQueue:     resQ,
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		BatchSize:       10,
	}
	var mu sync.Mutex
	ready := false
	svc := New(Options{})
	svc.mqSrc = func(context.Context) (mns.Config, int, bool) {
		mu.Lock()
		defer mu.Unlock()
		if !ready {
			return mns.Config{}, 1, false
		}
		return cfg, 1, true
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	client, err := mns.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Send(context.Background(), reqQ, []byte(`not-json`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	mem.mu.Lock()
	pending := len(mem.deleted)
	mem.mu.Unlock()
	if pending != 0 {
		t.Fatal("must not consume before mq env is ready")
	}

	mu.Lock()
	ready = true
	mu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mem.mu.Lock()
		deleted := len(mem.deleted)
		posted := len(mem.posted)
		mem.mu.Unlock()
		if deleted == 1 && posted >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	mem.mu.Lock()
	defer mem.mu.Unlock()
	t.Fatalf("deleted=%d posted=%d", len(mem.deleted), len(mem.posted))
}

func TestRunEnvConsumesEveryEnabledMQ(t *testing.T) {
	memA, srvA := startMemMNS(t, "REQ-A", "RES-A")
	memZ, srvZ := startMemMNS(t, "REQ-Z", "RES-Z")
	envs := setupMQEnvs(t)
	ctx := context.Background()
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	save := func(name, endpoint, req, res string) {
		t.Helper()
		_, err := envs.Save(ctx, actor, env.Env{
			WorkspaceRecID: access.DefaultWorkspaceRecID,
			EnvKind:        "mq",
			EnvName:        name,
			IsEnabled:      1,
			Config: map[string]any{
				"endpoint":          endpoint,
				"request_queue":     req,
				"result_queue":      res,
				"access_key_id":     "ak",
				"access_key_secret": "sk",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	save("mq-a", srvA.URL, "REQ-A", "RES-A")
	save("mq-z", srvZ.URL, "REQ-Z", "RES-Z")
	clientA, err := mns.New(mns.Config{Endpoint: srvA.URL, AccessKeyID: "ak", AccessKeySecret: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	clientZ, err := mns.New(mns.Config{Endpoint: srvZ.URL, AccessKeyID: "ak", AccessKeySecret: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clientA.Send(ctx, "REQ-A", []byte(`not-json`)); err != nil {
		t.Fatal(err)
	}
	if _, err := clientZ.Send(ctx, "REQ-Z", []byte(`not-json`)); err != nil {
		t.Fatal(err)
	}

	svc := New(Options{Envs: envs})
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := svc.Start(runCtx); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		memA.mu.Lock()
		delA, postA := len(memA.deleted), len(memA.posted)
		memA.mu.Unlock()
		memZ.mu.Lock()
		delZ, postZ := len(memZ.deleted), len(memZ.posted)
		memZ.mu.Unlock()
		if delA == 1 && delZ == 1 && postA >= 2 && postZ >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	memA.mu.Lock()
	memZ.mu.Lock()
	defer memA.mu.Unlock()
	defer memZ.mu.Unlock()
	t.Fatalf("a deleted=%d posted=%d; z deleted=%d posted=%d", len(memA.deleted), len(memA.posted), len(memZ.deleted), len(memZ.posted))
}

func setupMQEnvs(t *testing.T) *env.Service {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := user.SeedDefaultAdmin(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SeedDefaultWorkspace(ctx, st); err != nil {
		t.Fatal(err)
	}
	cipher, err := crypt.NewFromPassphrase("test-key")
	if err != nil {
		t.Fatal(err)
	}
	return env.NewService(st, iam.NewGuard(access.NewDirectory(st)), cipher)
}
