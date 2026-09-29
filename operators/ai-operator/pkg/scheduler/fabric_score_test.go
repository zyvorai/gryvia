package scheduler

import (
	"math"
	"testing"
)

func TestFabricPenalty(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"zero (empty status)", 0, 0},
		{"negative", -0.5, 0},
		{"NaN", math.NaN(), 0},
		{"quarter", 0.25, 6.25},
		{"one", 1, MaxFabricPenalty},
		{"above one is capped", 7, MaxFabricPenalty},
		{"+Inf is capped", math.Inf(1), MaxFabricPenalty},
		{"-Inf", math.Inf(-1), 0},
	}
	for _, c := range cases {
		if got := FabricPenalty(c.in); got != c.want {
			t.Errorf("%s: FabricPenalty(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
	if MaxFabricPenalty != 25 {
		t.Errorf("cap = %v, want 25", MaxFabricPenalty)
	}
}

func TestApplyFabricPenalty(t *testing.T) {
	if got := ApplyFabricPenalty(80, 0); got != 80 {
		t.Errorf("healthy fabric changed the score: %v", got)
	}
	if got := ApplyFabricPenalty(80, 0.4); got != 70 {
		t.Errorf("got %v, want 70", got)
	}
	if got := ApplyFabricPenalty(10, 1); got != 0 {
		t.Errorf("score must not go negative, got %v", got)
	}
	if got := ApplyFabricPenalty(0, 0.5); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}
