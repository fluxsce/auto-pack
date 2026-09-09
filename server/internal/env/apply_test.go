package env

import (
	"strings"
	"testing"
)

func TestProcEnvNilMaps(t *testing.T) {
	if env := ProcEnv(nil, nil, nil); len(env) != 0 {
		t.Fatal(env)
	}
}

func TestProcEnvSetsJavaAndRegistry(t *testing.T) {
	env := ProcEnv(
		map[string]any{"java_home": "/opt/jdk"},
		map[string]any{"maven_home": "/opt/maven"},
		map[string]any{"registry": "https://reg.example", "auth_token": "t1"},
	)
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "JAVA_HOME=/opt/jdk") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "MAVEN_HOME=/opt/maven") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "npm_config_registry=https://reg.example") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "PATH=") {
		t.Fatal(joined)
	}
}

func TestMavenCommandInjectsSettings(t *testing.T) {
	got := MavenCommand("mvn -B package", "/tmp/settings.xml")
	if !strings.Contains(got, "-s ") || !strings.Contains(got, "settings.xml") {
		t.Fatal(got)
	}
	keep := MavenCommand("mvn -s /a.xml package", "/tmp/settings.xml")
	if keep != "mvn -s /a.xml package" {
		t.Fatal(keep)
	}
}
