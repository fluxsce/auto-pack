package intake

import (
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

func TestParseProjectAndResources(t *testing.T) {
	raw := []byte(`{"customerCode":"LSYM","customerShortName":"零食有鸣","resourceFiles":"app.war, config.zip，lib.jar","mailTo":"zhangsan,lisi@corp.com"}`)
	n, err := parseNotice(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.CustomerCode != "LSYM" || n.CustomerShortName != "零食有鸣" || len(n.ResourceFiles) != 3 || n.ResourceFiles[0] != "app.war" || n.ResourceFiles[2] != "lib.jar" {
		t.Fatalf("%+v", n)
	}
	if len(n.Recipients) != 2 || n.Recipients[0] != "zhangsan@flux.com.cn" || n.Recipients[1] != "lisi@corp.com" {
		t.Fatalf("%v", n.Recipients)
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
