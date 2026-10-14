package intake

import "testing"

func TestReceiptMessageUsesTaskIDKey(t *testing.T) {
	body := []byte(`{"taskId":"TSKIDS-1","resultStatus":"SUCCESS"}`)
	msg := receiptMessage("DATAHUB_PACK_RECEIPT", body)
	if msg.Topic != "DATAHUB_PACK_RECEIPT" {
		t.Fatal(msg.Topic)
	}
	if msg.GetKeys() != "TSKIDS-1" {
		t.Fatal(msg.GetKeys())
	}
}

func TestSplitServers(t *testing.T) {
	got := splitServers(" 10.0.0.1:9876; 10.0.0.2:9876 ")
	if len(got) != 2 || got[0] != "10.0.0.1:9876" || got[1] != "10.0.0.2:9876" {
		t.Fatal(got)
	}
}

func TestOpenRocketRequiresTopics(t *testing.T) {
	_, err := OpenRocket(RocketConfig{NameServer: "127.0.0.1:9876"})
	if err != errNoTopic {
		t.Fatal(err)
	}
}
