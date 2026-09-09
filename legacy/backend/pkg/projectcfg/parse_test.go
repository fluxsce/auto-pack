package projectcfg

import "testing"

func TestNormalizeConfigJSON_emptyBecomesObject(t *testing.T) {
	got, err := NormalizeConfigJSON("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "{}" {
		t.Fatalf("want {} got %q", got)
	}

	got, err = NormalizeConfigJSON(`{"build_cmd":"","output_dir":"","artifact_pattern":""}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "{}" {
		t.Fatalf("want {} for all-empty fields, got %q", got)
	}
}

func TestNormalizeConfigJSON_preservesValues(t *testing.T) {
	raw := `{"build_cmd":"npm run build","output_dir":"dist","artifact_pattern":"*"}`
	got, err := NormalizeConfigJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfigJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BuildCmd != "npm run build" || cfg.OutputDir != "dist" || cfg.ArtifactPattern != "*" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
