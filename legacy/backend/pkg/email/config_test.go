package email

import "testing"

func TestConfigNormalizePorts(t *testing.T) {
	tests := []struct {
		port      int
		wantTLS   bool
		wantStart bool
	}{
		{465, true, false},
		{587, false, true},
		{25, false, true},
	}
	for _, tc := range tests {
		cfg := Config{Host: "smtp.example.com", Port: tc.port, From: "noreply@example.com"}
		cfg.Normalize()
		if cfg.TLS != tc.wantTLS {
			t.Fatalf("port %d: TLS=%v want %v", tc.port, cfg.TLS, tc.wantTLS)
		}
		if cfg.StartTLS != tc.wantStart {
			t.Fatalf("port %d: StartTLS=%v want %v", tc.port, cfg.StartTLS, tc.wantStart)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	_, err := New(Config{Host: "smtp.example.com", From: "not-an-email"})
	if err == nil {
		t.Fatal("expected invalid from")
	}
	_, err = New(Config{Host: "smtp.example.com", From: "a@b.com"})
	if err != nil {
		t.Fatal(err)
	}
}
