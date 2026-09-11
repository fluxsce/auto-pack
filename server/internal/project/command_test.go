package project

import (
	"errors"
	"testing"
)

func TestPatchCommandMavenGoal(t *testing.T) {
	got, ok := PatchCommand("mvn -B -DskipTests package", CommandPatch{Find: "package", Replace: "install"})
	if !ok || got != "mvn -B -DskipTests install" {
		t.Fatalf("%v %q", ok, got)
	}
	if _, ok := PatchCommand("mvn -B -DskipTests install", CommandPatch{Find: "package", Replace: "install"}); ok {
		t.Fatal("already install")
	}
}

func TestRewriteMavenGoalKeepsCleanAndFlags(t *testing.T) {
	got, ok := PatchCommand("mvn clean deploy -DskipTests", CommandPatch{Goal: "install"})
	if !ok || got != "mvn clean install -DskipTests" {
		t.Fatalf("%v %q", ok, got)
	}
	if _, ok := PatchCommand("mvn clean install -DskipTests", CommandPatch{Goal: "install"}); ok {
		t.Fatal("already install")
	}
}

func TestPatchCommandSet(t *testing.T) {
	got, ok := PatchCommand("npm run build", CommandPatch{SetCommand: "pnpm run build"})
	if !ok || got != "pnpm run build" {
		t.Fatalf("%v %q", ok, got)
	}
	got, ok = PatchCommand("yarn build", CommandPatch{Find: "npm", SetCommand: "pnpm run build"})
	if ok {
		t.Fatalf("should skip yarn: %q", got)
	}
}

func TestMatchCommandFamily(t *testing.T) {
	if !MatchCommandFamily("maven", "maven") || MatchCommandFamily("vue", "maven") {
		t.Fatal("maven")
	}
	if !MatchCommandFamily("vue", "node") || !MatchCommandFamily("npm", "node") || MatchCommandFamily("go", "node") {
		t.Fatal("node")
	}
}

func TestCheckCommandPatch(t *testing.T) {
	if err := CheckCommandPatch(CommandPatch{Find: "package", Replace: "install"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckCommandPatch(CommandPatch{SetCommand: "mvn -B install"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckCommandPatch(CommandPatch{Family: "go"}); !errors.Is(err, ErrCommandFamily) {
		t.Fatalf("%v", err)
	}
	if err := CheckCommandPatch(CommandPatch{Find: "package"}); !errors.Is(err, ErrCommandPatch) {
		t.Fatalf("%v", err)
	}
	if err := CheckCommandPatch(CommandPatch{Goal: "compile"}); !errors.Is(err, ErrCommandGoal) {
		t.Fatalf("%v", err)
	}
	if err := CheckCommandPatch(CommandPatch{Goal: "install"}); err != nil {
		t.Fatal(err)
	}
}
