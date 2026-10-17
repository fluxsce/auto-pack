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

func TestComposePackMailAttachesLog(t *testing.T) {
	got, err := ComposePackMail(PackMailFields{
		OK: true, TaskID: "T1", Project: "dh", Branch: "release/V9",
		Result: "自动打包完成", Remote: "/开发/S01/app.war",
		URL: "https://oss.example/app.war", Log: "BUILD SUCCESS",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Subject, "打包已完成") || !strings.Contains(got.Text, "BUILD SUCCESS") || !strings.Contains(got.HTML, "BUILD SUCCESS") {
		t.Fatalf("%s\n%s", got.Subject, got.Text)
	}
}

func TestComposeMqMailOSSUpdateShowsBackup(t *testing.T) {
	got, err := ComposeMqMail(MailTemplateOSSUpdate, MqMailFields{
		Project:   "零食有鸣",
		TaskNo:    "TSK-0001",
		Resources: "app.war, config.zip",
		Remote:    "/开发/S01/app.war",
		URL:       "https://oss.example/app.war",
		Until:     "2026-10-17 23:00:00",
		Branch:    "release/customer",
		Backup:    "/开发/S01/BAK/20261017004900123/app.war",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "【零食有鸣】申请更新TSK-0001, OSS已更新" || got.FromName != "DATAHUB 更新通知" {
		t.Fatalf("%+v", got)
	}
	for _, part := range []string{"更新内容：【零食有鸣】申请更新TSK-0001", "任务编号：TSK-0001", "OSS访问路径：https://oss.example/app.war", "更新war包：app.war, config.zip", "更新范围：DATAHUB服务", "备份：/开发/S01/BAK/20261017004900123/app.war", "有效期至 2026-10-17 23:00:00"} {
		if !strings.Contains(got.Text, part) {
			t.Fatalf("missing %q in %s", part, got.Text)
		}
	}
	if strings.Contains(got.Text, "FTP已更新") || strings.Contains(got.Text, "构建日志") || !strings.Contains(got.HTML, "OSS已更新") {
		t.Fatalf("%s", got.Text)
	}
}

func TestComposeMqMailKeepsFTPSource(t *testing.T) {
	got, err := ComposeMqMail(MailTemplateFTPUpdate, MqMailFields{
		Project:   "零食有鸣",
		TaskNo:    "TSK-0001",
		Resources: "app.war",
		Remote:    "/ftp/S01/app.war",
		Backup:    "/ftp/S01/BAK/20261017004900123/app.war",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Subject, "FTP已更新") || !strings.Contains(got.Text, "FTP版本来源：/ftp/S01/app.war") || !strings.Contains(got.Text, "更新范围：DATAHUB服务") || !strings.Contains(got.Text, "备份：/ftp/S01/BAK/20261017004900123/app.war") {
		t.Fatalf("%s\n%s", got.Subject, got.Text)
	}
	if strings.Contains(got.Text, "OSS访问路径") || strings.Contains(got.Subject, "OSS已更新") {
		t.Fatalf("%s", got.Text)
	}
}

func TestComposeUpdateMailUnknown(t *testing.T) {
	_, err := ComposeUpdateMail("nope", UpdateMailFields{}, time.Now())
	if err != ErrMailTemplate {
		t.Fatalf("err=%v", err)
	}
}
