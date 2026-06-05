package workspaceftppush

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultBackupPath(t *testing.T) {
	ts := time.Date(2026, 6, 4, 14, 30, 5, 0, time.UTC)
	got := DefaultBackupPath("/deploy/release", ts)
	want := "/deploy/release/BAK/20260604143005000"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolvePushLayout(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		got := ResolvePushLayout("/deploy/release", "pom.xml")
		want := PushLayout{PushDir: "/deploy/release", UpdatePath: "/deploy/release/pom.xml"}
		if got != want {
			t.Fatalf("got %+v want %+v", got, want)
		}
	})
	t.Run("file path", func(t *testing.T) {
		got := ResolvePushLayout("/开发/1测试项目/pom.xml", "pom.xml")
		want := PushLayout{PushDir: "/开发/1测试项目", UpdatePath: "/开发/1测试项目/pom.xml"}
		if got != want {
			t.Fatalf("got %+v want %+v", got, want)
		}
	})
}

func TestMergeUpdateNoteContent(t *testing.T) {
	existing := "20260529 shangjian old entry\n备份包路径:/bak/old.war\n更新包路径:/deploy/old.war\n"
	manifest := "20260604 shangjian new entry\n备份包路径:/bak/new.war\n更新包路径:/deploy/new.war\n"
	got := MergeUpdateNoteContent(existing, manifest)
	want := strings.TrimRight(existing, "\n") + "\n" + strings.TrimRight(manifest, "\n") + "\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildManifestContent(t *testing.T) {
	ts := time.Date(2026, 5, 29, 14, 30, 0, 0, time.UTC)
	detail := "    1.flux-collaboration-storage-dataoperation\n        src/main/java/com/flux/collaboration/storage/base/cache/MemRecordParam.java\n    2.flux-collaboration-utils\n        src/main/java/com/flux/collaboration/utils/reference/DatahubList.java"
	got := BuildManifestContent(ManifestInput{
		Pusher:   "shangjian",
		TaskRef:  "响应报文超过2万行时，JDBC OUT组件的std只有2万条数据,客服号:20260525110238000736",
		Detail:   detail,
		PushTime: ts,
	}, "/开发/石头科技/datahubV9-P06/BAK/20260529/datahubServer.war", "/开发/石头科技/datahubV9-P06/datahubServer.war")
	want := "20260529 shangjian 响应报文超过2万行时，JDBC OUT组件的std只有2万条数据,客服号:20260525110238000736\n" +
		"备份包路径:/开发/石头科技/datahubV9-P06/BAK/20260529/datahubServer.war\n" +
		"更新包路径:/开发/石头科技/datahubV9-P06/datahubServer.war\n" +
		detail + "\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
