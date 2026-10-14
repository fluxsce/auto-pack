package env

import (
	"strings"
	"testing"
	"time"
)

func TestComposeUpdateMailFTPUpdate(t *testing.T) {
	at := time.Date(2026, 6, 17, 10, 0, 0, 0, time.Local)
	got, err := ComposeUpdateMail(MailTemplateFTPUpdate, UpdateMailFields{
		ProjectLabel:     "NIKECLC",
		UpdateContent:    "申请更新1",
		FTPVersionSource: "/deploy/update",
		WarPackage:       "app.war、ui.war",
		Backup:           "/BAK/app.war",
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "【NIKECLC】申请更新1, FTP已更新" {
		t.Fatalf("subject=%q", got.Subject)
	}
	if !strings.Contains(got.Text, "上午好。【NIKECLC】申请更新1, FTP已更新") {
		t.Fatalf("text=%q", got.Text)
	}
	if !strings.Contains(got.HTML, "DATAHUB 更新通知") {
		t.Fatalf("html missing brand: %s", got.HTML)
	}
	if !strings.Contains(got.HTML, "FTP已更新") {
		t.Fatalf("html missing status: %s", got.HTML)
	}
	if got.FromName != "DATAHUB 更新通知" {
		t.Fatalf("fromName=%q", got.FromName)
	}
}

func TestComposeUpdateMailProjectUpdate(t *testing.T) {
	at := time.Date(2026, 6, 17, 15, 0, 0, 0, time.Local)
	got, err := ComposeUpdateMail(MailTemplateProjectUpdate, UpdateMailFields{
		ProjectLabel:  "NIKECLC",
		UpdateContent: "申请更新手工补丁",
		UpdateScope:   "DATAHUB服务",
		WarPackage:    "svc.war",
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "【NIKECLC】申请更新手工补丁, 项目已更新" {
		t.Fatalf("subject=%q", got.Subject)
	}
	if !strings.Contains(got.Text, "下午好。【NIKECLC】申请更新手工补丁, 项目已更新") {
		t.Fatalf("text=%q", got.Text)
	}
	if !strings.Contains(got.HTML, "项目更新通知") {
		t.Fatalf("html missing brand: %s", got.HTML)
	}
	if !strings.Contains(got.HTML, "项目已更新") {
		t.Fatalf("html missing status: %s", got.HTML)
	}
	if got.FromName != "项目更新通知" {
		t.Fatalf("fromName=%q", got.FromName)
	}
}

func TestComposeUpdateMailUnknown(t *testing.T) {
	_, err := ComposeUpdateMail("nope", UpdateMailFields{}, time.Now())
	if err != ErrMailTemplate {
		t.Fatalf("err=%v", err)
	}
}
