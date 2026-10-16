package env

import (
	"strings"
	"testing"

	hostmns "auto-pack/server/internal/host/mns"
)

func TestResolveMQTestQueue(t *testing.T) {
	mc := hostmns.Config{RequestQueue: "REQ", ResultQueue: "RES"}
	name, label, err := resolveMQTestQueue(mc, "request")
	if err != nil || name != "REQ" || label != "request" {
		t.Fatalf("%s %s %v", name, label, err)
	}
	name, label, err = resolveMQTestQueue(mc, "result")
	if err != nil || name != "RES" || label != "result" {
		t.Fatalf("%s %s %v", name, label, err)
	}
	if _, _, err := resolveMQTestQueue(mc, "other"); err == nil {
		t.Fatal("want error")
	}
	if _, _, err := resolveMQTestQueue(hostmns.Config{}, "request"); err == nil {
		t.Fatal("empty queue")
	}
}

func TestEmptyQueueMessage(t *testing.T) {
	if !strings.Contains(emptyQueueMessage("request"), "intake") {
		t.Fatal(emptyQueueMessage("request"))
	}
	if emptyQueueMessage("result") != "queue empty" {
		t.Fatal(emptyQueueMessage("result"))
	}
}
