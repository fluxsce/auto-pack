package oss

import (
	"io"
	"net/http"
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
	got, err := signGetAt(cfg, "开发/S01/app.war", exp)
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

func TestSignGetRejectsBlank(t *testing.T) {
	if _, err := SignGet(Config{}, "a.war", time.Hour); err == nil {
		t.Fatal("blank config")
	}
}
