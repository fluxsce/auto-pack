package bash

import (
	"runtime"
	"testing"
)

func TestShellArgsAvoidsLoginProfile(t *testing.T) {
	name, args := shellArgs("mvn -v")
	if runtime.GOOS == "windows" {
		if name != "cmd" || len(args) != 2 || args[0] != "/c" || args[1] != "mvn -v" {
			t.Fatalf("%s %v", name, args)
		}
		return
	}
	if name != "bash" {
		t.Fatal(name)
	}
	joined := args[0] + " " + args[1] + " " + args[2]
	if joined != "--noprofile --norc -c" || args[3] != "mvn -v" {
		t.Fatal(args)
	}
	for _, a := range args {
		if a == "-l" || a == "-lc" {
			t.Fatalf("login shell would reset JAVA_HOME: %v", args)
		}
	}
}
