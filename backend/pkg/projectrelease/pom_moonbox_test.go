package projectrelease

import (
	"strings"
	"testing"
)

func TestUpdatePomVersions_moonboxApiParentBeforeModelVersion(t *testing.T) {
	const pom = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <parent>
        <groupId>com.flux.wmsv6</groupId>
        <artifactId>flux-collaboration-storage-repeater-parent</artifactId>
        <version>V9.0.0-P08-01-SNAPSHOT</version>
    </parent>
    <modelVersion>4.0.0</modelVersion>
    <artifactId>moonbox-api</artifactId>

    <dependencies>
        <dependency>
            <groupId>com.alibaba.jvm.sandbox</groupId>
            <artifactId>sandbox-api</artifactId>
            <version>1.4.0</version>
            <scope>provided</scope>
        </dependency>
        <dependency>
            <groupId>com.flux.wmsv6</groupId>
            <artifactId>moonbox-tools</artifactId>
            <version>${datahub.core.version}</version>
        </dependency>
    </dependencies>
</project>`
	tag := "V9.0.0-P09"
	pomVer := PomVersionForTag(tag)
	out, skip, err := updatePomVersions(pom, tag, "flux-collaboration-storage-repeater-parent")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>"+pomVer+"</version>") {
		t.Fatalf("parent version should update to %s: %s", pomVer, out)
	}
	if !strings.Contains(out, "<version>1.4.0</version>") {
		t.Fatalf("sandbox-api must stay 1.4.0: %s", out)
	}
	if strings.Contains(out, "sandbox-api") && strings.Contains(out, "<version>"+pomVer+"</version>") {
		// parent has pomVer; ensure sandbox block does not
		idx := strings.Index(out, "sandbox-api")
		chunk := out[idx : idx+200]
		if strings.Contains(chunk, "<version>"+pomVer+"</version>") {
			t.Fatalf("must not rewrite sandbox-api version: %s", chunk)
		}
	}
}
