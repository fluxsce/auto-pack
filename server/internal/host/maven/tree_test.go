package maven

import (
	"strings"
	"testing"
)

func TestParseTreeMultiModule(t *testing.T) {
	raw := `
[INFO] Scanning for projects...
[INFO] ---------------------< com.acme:parent >----------------------
[INFO] Building parent 1.2.0
[INFO] --- maven-dependency-plugin:3.6.1:tree (default-cli) @ parent ---
[INFO] com.acme:parent:pom:1.2.0
[INFO] +- org.slf4j:slf4j-api:jar:2.0.9:compile
[INFO] 
[INFO] ---------------------< com.acme:app >----------------------
[INFO] Building app 1.2.0
[INFO] --- maven-dependency-plugin:3.6.1:tree (default-cli) @ app ---
[INFO] com.acme:app:jar:1.2.0
[INFO] +- com.google.guava:guava:jar:32.0.0-jre:compile
[INFO] |  +- com.google.guava:failureaccess:jar:1.0.1:compile
[INFO] |  \- com.google.code.findbugs:jsr305:jar:3.0.2:compile
[INFO] +- org.apache.commons:commons-lang3:jar:3.14.0:compile (version managed from 3.12.0)
[INFO] \- junit:junit:jar:4.13.2:test
[INFO] BUILD SUCCESS
`
	nodes := ParseTree(raw)
	if len(nodes) < 7 {
		t.Fatalf("nodes=%d %+v", len(nodes), nodes)
	}
	if nodes[0].Name != "com.acme:parent" || nodes[0].Extra != "module" || nodes[0].Depth != 0 {
		t.Fatalf("root=%+v", nodes[0])
	}
	var guava, jsr, lang, test *Node
	for i := range nodes {
		n := &nodes[i]
		switch n.Name {
		case "com.google.guava:guava":
			guava = n
		case "com.google.code.findbugs:jsr305":
			jsr = n
		case "org.apache.commons:commons-lang3":
			lang = n
		case "junit:junit":
			test = n
		}
	}
	if guava == nil || guava.Version != "32.0.0-jre" || guava.Depth != 1 || guava.Source != "com.acme:app:1.2.0" {
		t.Fatalf("guava=%+v", guava)
	}
	if jsr == nil || jsr.Depth != 2 {
		t.Fatalf("jsr=%+v", jsr)
	}
	if lang == nil || lang.Version != "3.14.0" {
		t.Fatalf("lang=%+v", lang)
	}
	if test == nil || test.Extra != "test" {
		t.Fatalf("test=%+v", test)
	}
}

func TestParseCoordClassifier(t *testing.T) {
	c, ok := parseCoord("org.foo:bar:jar:sources:1.2.3:compile")
	if !ok || c.name != "org.foo:bar:sources" || c.version != "1.2.3" || c.extra != "compile" {
		t.Fatalf("%+v ok=%v", c, ok)
	}
}

func TestParseJSONAndConflicts(t *testing.T) {
	raw := []byte(`{
  "groupId": "com.acme",
  "artifactId": "app",
  "version": "1.2.0",
  "type": "jar",
  "scope": "",
  "classifier": "",
  "optional": "false",
  "children": [
    {
      "groupId": "com.google.guava",
      "artifactId": "guava",
      "version": "32.0.0-jre",
      "type": "jar",
      "scope": "compile",
      "classifier": "",
      "optional": "false",
      "children": [
        {
          "groupId": "org.checkerframework",
          "artifactId": "checker-qual",
          "version": "3.33.0",
          "type": "jar",
          "scope": "compile",
          "classifier": "",
          "optional": "false"
        }
      ]
    },
    {
      "groupId": "org.checkerframework",
      "artifactId": "checker-qual",
      "version": "3.12.0",
      "type": "jar",
      "scope": "compile",
      "classifier": "",
      "optional": "false"
    }
  ]
}`)
	nodes := ParseJSON(raw)
	annotateConflicts(nodes)
	if len(nodes) != 4 {
		t.Fatalf("n=%d", len(nodes))
	}
	if nodes[0].Extra != "module" || nodes[1].Depth != 1 {
		t.Fatalf("%+v", nodes[0])
	}
	var conflict, winner *Node
	for i := range nodes {
		if nodes[i].Name != "org.checkerframework:checker-qual" {
			continue
		}
		if nodes[i].Extra == "conflict" {
			conflict = &nodes[i]
		} else {
			winner = &nodes[i]
		}
	}
	if winner == nil || winner.Version != "3.12.0" || winner.Depth != 1 {
		t.Fatalf("winner=%+v", winner)
	}
	if conflict == nil || conflict.Version != "3.33.0" || !strings.Contains(conflict.Note, "3.12.0") {
		t.Fatalf("conflict=%+v nodes=%+v", conflict, nodes)
	}
}
