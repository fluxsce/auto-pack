package projectrelease

import "testing"

func TestMarshalParsePostOptions(t *testing.T) {
	in := &PostOptions{
		AutoBuildAfterRelease: true,
		AutoFtpPushAfterBuild: true,
		FtpPushProjectIDs:     []string{"p1", "p2"},
		FtpEnvRecID:           "env-1",
		PushPath:              "/update",
		Pusher:                "alice",
		TaskRef:               "V9.0.0-P01-01",
		UpdatePomVersion:      boolPtr(true),
		CreateTargetBranch:    boolPtr(false),
	}
	raw, err := MarshalPostOptions(in)
	if err != nil || raw == "" {
		t.Fatalf("marshal: %v %q", err, raw)
	}
	out, err := ParsePostOptionsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.FtpPushProjectIDs) != 2 || out.PushPath != "/update" {
		t.Fatalf("got %+v", out)
	}
}

func boolPtr(b bool) *bool { return &b }
