package httpx

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestWriteErrorLogsClientAndServerErrors(t *testing.T) {
	buf := captureLog(t)
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "must be letters, digits, underscore or hyphen")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(buf.String(), "http 400:") || !strings.Contains(buf.String(), "must be letters") {
		t.Fatalf("log=%q", buf.String())
	}
}

func TestWriteErrorSkipsUnauthorized(t *testing.T) {
	buf := captureLog(t)
	WriteError(httptest.NewRecorder(), http.StatusUnauthorized, "missing bearer token")
	if buf.Len() != 0 {
		t.Fatalf("log=%q", buf.String())
	}
}

func TestWithRecoverLogsPanic(t *testing.T) {
	buf := captureLog(t)
	h := WithRecover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(buf.String(), "panic GET /api/x") || !strings.Contains(buf.String(), "boom") {
		t.Fatalf("log=%q", buf.String())
	}
}
