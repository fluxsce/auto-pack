package projects

import (
	"net/url"
	"testing"
)

func TestCollectDownloadPaths(t *testing.T) {
	q := url.Values{}
	q.Add("path", "src/a.txt")
	q.Add("path", " src/a.txt ")
	q.Add("path", "pom.xml")
	q.Add("path[]", "lib/b.js")
	q.Add("path", "")
	got := collectDownloadPaths(q)
	if len(got) != 3 || got[0] != "src/a.txt" || got[1] != "pom.xml" || got[2] != "lib/b.js" {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectDownloadPathsEmpty(t *testing.T) {
	if got := collectDownloadPaths(url.Values{}); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}
