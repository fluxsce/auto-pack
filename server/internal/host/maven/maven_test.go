package maven

import "testing"

func TestUpdateProjectVersion(t *testing.T) {
	in := `<project><modelVersion>4.0.0</modelVersion><version>1.0.0</version></project>`
	out, err := Update(in, "V9.0.0-P01-SNAPSHOT")
	if err != nil {
		t.Fatal(err)
	}
	if out == in {
		t.Fatal("expected rewrite")
	}
}

func TestSnapshot(t *testing.T) {
	if Snapshot("V1") != "V1-SNAPSHOT" {
		t.Fatal(Snapshot("V1"))
	}
}
