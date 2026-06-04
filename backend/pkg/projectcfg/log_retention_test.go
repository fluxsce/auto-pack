package projectcfg

import "testing"

func TestEffectiveLogRetentionDays(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, DefaultLogRetentionDays},
		{-1, DefaultLogRetentionDays},
		{3, 3},
		{1, 1},
		{365, 365},
		{999, maxLogRetentionDays},
	}
	for _, tc := range tests {
		if got := EffectiveLogRetentionDays(tc.in); got != tc.want {
			t.Fatalf("EffectiveLogRetentionDays(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
