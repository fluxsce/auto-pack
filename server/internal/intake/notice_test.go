package intake

import (
	"context"
	"strings"
	"testing"
)

func TestValidateDataHub(t *testing.T) {
	ok := sampleNotice()
	if validateDataHub(ok) != "" {
		t.Fatal(validateDataHub(ok))
	}
	legacy := ok
	legacy.Legacy = true
	if !strings.Contains(validateDataHub(legacy), "旧报文") {
		t.Fatal(validateDataHub(legacy))
	}
	badMode := ok
	badMode.PackagingMode = "jar"
	if !strings.Contains(validateDataHub(badMode), "war") {
		t.Fatal(validateDataHub(badMode))
	}
	branch := ok
	branch.BaseRefType = "branch"
	if !strings.Contains(validateDataHub(branch), "tag") {
		t.Fatal(validateDataHub(branch))
	}
	noPath := ok
	noPath.OSSProjectRoot = ""
	if !strings.Contains(validateDataHub(noPath), "ossProjectRoot") {
		t.Fatal(validateDataHub(noPath))
	}
	ftpOnly := noPath
	ftpOnly.FTPProjectRoot = "/ftp/S01/"
	if validateDataHub(ftpOnly) != "" {
		t.Fatal(validateDataHub(ftpOnly))
	}
}

func TestParseTransferPath(t *testing.T) {
	raw := []byte(`{"ossProjectRoot":"/开发/S01/","ftpProjectRoot":"/ftp/S01/","endpoint":"https://should-not-use","accessKeyId":"ak"}`)
	n, err := parseNotice(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Legacy || n.OSSProjectRoot != "/开发/S01/" || n.FTPProjectRoot != "/ftp/S01/" {
		t.Fatalf("%+v", n)
	}
}

func TestParseNoticeRecipients(t *testing.T) {
	raw := []byte(`{"taskId":"T1","emails":["a@b.c","c@d.e"],"mailTo":"a@b.c"}`)
	n, err := parseNotice(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Recipients) != 2 || n.Recipients[0] != "a@b.c" || n.Recipients[1] != "c@d.e" {
		t.Fatalf("%v", n.Recipients)
	}
}

func TestLoadMQConfig(t *testing.T) {
	got := fromConfig(map[string]any{
		"project_id":      "p-dh",
		"name_server":     "10.0.0.1:9876",
		"notice_topic":    "DATAHUB_PACK_NOTICE",
		"receipt_topic":   "DATAHUB_PACK_RECEIPT",
		"receipt_retries": float64(5),
	})
	if !got.Enabled || got.ProjectID != "p-dh" || got.Rocket.NoticeTopic != "DATAHUB_PACK_NOTICE" || got.Retries != 5 {
		t.Fatalf("%+v", got)
	}
	if fromConfig(map[string]any{"name_server": "10.0.0.1:9876"}).Enabled {
		t.Fatal("incomplete")
	}
	if Load(context.Background(), nil).Enabled {
		t.Fatal("nil env")
	}
}

func TestParseNoticeLegacyField(t *testing.T) {
	raw := []byte(`{"taskId":"T1","sourceMetadata":{},"SYSVAL":"V10DATAHUB"}`)
	n, err := parseNotice(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !n.Legacy || n.SYSVAL != "V10DATAHUB" || n.TaskID != "T1" {
		t.Fatalf("%+v", n)
	}
}

func TestPackCommandKeepsParams(t *testing.T) {
	got := packCommand("mvn -B package", "C01", "1.2.3", "V10DATAHUB")
	for _, part := range []string{"SYSVAL", "V10DATAHUB", "customerCode", "C01", "version", "1.2.3", "mvn -B package"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in %s", part, got)
		}
	}
	if packCommand("  ", "a", "b", "c") != "" {
		t.Fatal("empty command should stay empty")
	}
}

func TestRemoteWarUsesMessageRoot(t *testing.T) {
	got := remoteWar("/开发/S01/", "app.war")
	if got != "/开发/S01/app.war" {
		t.Fatal(got)
	}
}

func sampleNotice() Notice {
	return Notice{
		TaskID:             "TSKIDS-1",
		TaskNo:             "1",
		PackTaskID:         "PACK-1",
		SubsystemType:      subsystemDataHub,
		PackagingMode:      "war",
		DeploymentStrategy: "war",
		OSSProjectRoot:     "/开发/S01/",
		CustomerCode:       "C01",
		Version:            "1.2.3",
		CustomerBranch:     "release/customer",
		BaseRef:            "v1.2.3",
		BaseRefType:        "tag",
		SYSVAL:             "V10DATAHUB",
		Recipients:         []string{"a@b.c", "c@d.e"},
	}
}
