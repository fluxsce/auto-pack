package oss

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignGetUsesMessageCredential(t *testing.T) {
	cfg := Config{
		Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "datahub",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		SecurityToken:   "sts",
	}
	exp := time.Unix(1_800_000_000, 0)
	got, err := signGetAt(cfg, "开发/S01/app.war", exp, "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "datahub.oss-cn-hangzhou.aliyuncs.com" || u.Path != "/开发/S01/app.war" {
		t.Fatalf("%s", got)
	}
	q := u.Query()
	if q.Get("OSSAccessKeyId") != "ak" || q.Get("Expires") != "1800000000" || q.Get("security-token") != "sts" || q.Get("Signature") == "" {
		t.Fatalf("%v", q)
	}
	want := sign("sk", "GET\n\n\n1800000000\n/datahub/开发/S01/app.war?security-token=sts")
	if q.Get("Signature") != want {
		t.Fatalf("sig %s", q.Get("Signature"))
	}
}

func TestPutSignsDate(t *testing.T) {
	cfg := Config{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "datahub", AccessKeyID: "ak", AccessKeySecret: "sk"}
	now := time.Date(2026, 10, 14, 6, 0, 0, 0, time.UTC)
	req, err := newPut(cfg, "开发/S01/app.war", strings.NewReader("war"), 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPut || req.URL.Host != "datahub.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("%s %s", req.Method, req.URL)
	}
	date := now.Format(http.TimeFormat)
	want := "OSS ak:" + sign("sk", "PUT\n\napplication/octet-stream\n"+date+"\n/datahub/开发/S01/app.war")
	if req.Header.Get("Authorization") != want {
		t.Fatal(req.Header.Get("Authorization"))
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != "war" {
		t.Fatal(string(body))
	}
}

func TestMapConfigUsesPlatformEnv(t *testing.T) {
	cfg := MapConfig(map[string]any{
		"endpoint":          "https://oss-cn-hangzhou.aliyuncs.com",
		"bucket":            "datahub",
		"access_key_id":     "ak",
		"access_key_secret": "sk",
		"expire_hours":      float64(12),
	})
	if cfg.Endpoint == "" || cfg.Bucket != "datahub" || cfg.AccessKeyID != "ak" || cfg.AccessKeySecret != "sk" {
		t.Fatalf("%+v", cfg)
	}
	if ExpireHours(map[string]any{"expire_hours": float64(12)}) != 12 {
		t.Fatal("hours")
	}
	if ExpireHours(nil) != 24 {
		t.Fatal("default")
	}
}

func TestNewCopySignsSource(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 49, 0, 0, time.UTC)
	cfg := Config{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "datahub",
		AccessKeyID: "ak", AccessKeySecret: "sk",
	}
	req, err := newCopy(cfg, "开发/S01/app.war", "开发/S01/BAK/20261017004900123/app.war", now)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPut {
		t.Fatal(req.Method)
	}
	source := req.Header.Get("x-oss-copy-source")
	if source != "/datahub/"+url.PathEscape("开发")+"/S01/app.war" {
		t.Fatal(source)
	}
	date := now.Format(http.TimeFormat)
	want := "OSS ak:" + sign("sk", "PUT\n\n\n"+date+"\nx-oss-copy-source:"+source+"\n/datahub/开发/S01/BAK/20261017004900123/app.war")
	if req.Header.Get("Authorization") != want {
		t.Fatal(req.Header.Get("Authorization"))
	}
}

func TestExistsTreatsMissingObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("%s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, u.Host)
		},
	}}
	ok, err := existsWithClient(client, Config{
		Endpoint: "http://oss.example.test", Bucket: "datahub",
		AccessKeyID: "ak", AccessKeySecret: "sk",
	}, "开发/S01/app.war")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestSignGetRejectsBlank(t *testing.T) {
	if _, err := SignGet(Config{}, "a.war", time.Hour); err == nil {
		t.Fatal("blank config")
	}
}

func TestNewHeadSignsBucket(t *testing.T) {
	now := time.Date(2026, 10, 14, 6, 0, 0, 0, time.UTC)
	req, err := newHead(Config{
		Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "datahub",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodHead || req.URL.Host != "datahub.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("%s %s", req.Method, req.URL)
	}
	want := "OSS ak:" + sign("sk", "HEAD\n\n\n"+now.Format(http.TimeFormat)+"\n/datahub/")
	if req.Header.Get("Authorization") != want {
		t.Fatal(req.Header.Get("Authorization"))
	}
}

func TestProbeHeadBucketOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS ak:") {
			t.Fatalf("auth %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, u.Host)
			},
		},
	}
	err = probeWithClient(context.Background(), client, Config{
		Endpoint:        "http://oss.example.test",
		Bucket:          "datahub",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListSignsPrefix(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	req, err := newList(Config{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "datahub",
		AccessKeyID: "ak", AccessKeySecret: "sk",
	}, "开发/", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Host != "datahub.oss-cn-hangzhou.aliyuncs.com" || req.URL.Query().Get("prefix") != "开发/" {
		t.Fatalf("%s", req.URL)
	}
	date := now.Format(http.TimeFormat)
	want := "OSS ak:" + sign("sk", "GET\n\n\n"+date+"\n/datahub/?delimiter=/&max-keys=100&prefix=开发/")
	if req.Header.Get("Authorization") != want {
		t.Fatal(req.Header.Get("Authorization"))
	}
}

func TestParseListSkipsFolderPlaceholder(t *testing.T) {
	raw := []byte(`<ListBucketResult>
		<IsTruncated>true</IsTruncated><NextMarker>开发/b</NextMarker>
		<CommonPrefixes><Prefix>开发/dir/</Prefix></CommonPrefixes>
		<Contents><Key>开发/</Key><Size>0</Size></Contents>
		<Contents><Key>开发/app.war</Key><Size>4</Size><LastModified>2026-10-16T00:00:00.000Z</LastModified></Contents>
	</ListBucketResult>`)
	page, err := parseList("开发/", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Truncated || page.NextMarker != "开发/b" || len(page.Entries) != 2 {
		t.Fatalf("%+v", page)
	}
	if !page.Entries[0].Dir || page.Entries[0].Name != "dir" || page.Entries[1].Name != "app.war" || page.Entries[1].Size != 4 {
		t.Fatalf("%+v", page.Entries)
	}
}

func TestSignAttachmentNamesFile(t *testing.T) {
	cfg := Config{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "datahub",
		AccessKeyID: "ak", AccessKeySecret: "sk", SecurityToken: "sts",
	}
	exp := time.Unix(1_800_000_000, 0)
	got, err := signGetAt(cfg, "开发/S01/app.war", exp, "attachment")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	disp := u.Query().Get("response-content-disposition")
	if !strings.Contains(disp, "attachment") || !strings.Contains(disp, "app.war") {
		t.Fatalf("%s", disp)
	}
	resource := "/datahub/开发/S01/app.war?response-content-disposition=" + url.QueryEscape(disp) + "&security-token=sts"
	want := sign("sk", "GET\n\n\n1800000000\n"+resource)
	if u.Query().Get("Signature") != want {
		t.Fatalf("sig %s", u.Query().Get("Signature"))
	}
}

func TestProbeHeadBucketForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "AccessDenied", http.StatusForbidden)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, u.Host)
			},
		},
	}
	err = probeWithClient(context.Background(), client, Config{
		Endpoint:        "http://oss.example.test",
		Bucket:          "datahub",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("err=%v", err)
	}
}
