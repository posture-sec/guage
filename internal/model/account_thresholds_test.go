package model_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestThresholdsValid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   model.Thresholds
		want bool
	}{
		{"all zero", model.Thresholds{}, true},
		{"inside the range", model.Thresholds{Low: 1, Medium: 2, High: 3}, true},
		{"every tier at the top", model.Thresholds{Low: 255, Medium: 255, High: 255}, true},
		{"low below the range", model.Thresholds{Low: -1}, false},
		{"medium below the range", model.Thresholds{Medium: -1}, false},
		{"high below the range", model.Thresholds{High: -1}, false},
		{"low above the range", model.Thresholds{Low: 256}, false},
		{"medium above the range", model.Thresholds{Medium: 256}, false},
		{"high above the range", model.Thresholds{High: 256}, false},
		{"a decode that read 300", model.Thresholds{Low: 1, Medium: 1, High: 300}, false},
		{"one bad tier among good ones", model.Thresholds{Low: 255, Medium: -1, High: 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Valid(); got != tt.want {
				t.Fatalf("%+v.Valid() = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestThresholdsEqual(t *testing.T) {
	t.Parallel()
	base := model.Thresholds{Low: 1, Medium: 2, High: 3}
	for _, tt := range []struct {
		name  string
		other model.Thresholds
		want  bool
	}{
		{"the same values", model.Thresholds{Low: 1, Medium: 2, High: 3}, true},
		{"low differs", model.Thresholds{Low: 0, Medium: 2, High: 3}, false},
		{"medium differs", model.Thresholds{Low: 1, Medium: 0, High: 3}, false},
		{"high differs", model.Thresholds{Low: 1, Medium: 2, High: 0}, false},
		{"the same numbers in another order", model.Thresholds{Low: 3, Medium: 2, High: 1}, false},
		{"zero value", model.Thresholds{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := base.Equal(tt.other); got != tt.want {
				t.Fatalf("%+v.Equal(%+v) = %v, want %v", base, tt.other, got, tt.want)
			}
			if got := tt.other.Equal(base); got != tt.want {
				t.Fatalf("%+v.Equal(%+v) = %v, want %v (not symmetric)", tt.other, base, got, tt.want)
			}
		})
	}
	if !(model.Thresholds{}).Equal(model.Thresholds{}) {
		t.Fatal("two zero values are not equal")
	}
}

// flagCombination builds the flags for bit pattern n, so a sweep of 0 to 15 visits every
// combination of the four booleans exactly once.
func flagCombination(n int) model.Flags {
	return model.Flags{
		AuthRequired:    n&1 != 0,
		AuthRevocable:   n&2 != 0,
		AuthImmutable:   n&4 != 0,
		ClawbackEnabled: n&8 != 0,
	}
}

// Valid is true for every combination on purpose: each bool is in range and no
// exclusion between flags is documented. The sweep pins that, so adding a rule here
// is a visible decision rather than a silent change.
func TestFlagsValid(t *testing.T) {
	t.Parallel()
	for n := 0; n < 16; n++ {
		if f := flagCombination(n); !f.Valid() {
			t.Fatalf("%+v.Valid() = false, want true for every combination", f)
		}
	}
}

func TestFlagsEqual(t *testing.T) {
	t.Parallel()
	for i := 0; i < 16; i++ {
		for j := 0; j < 16; j++ {
			a, b := flagCombination(i), flagCombination(j)
			if got, want := a.Equal(b), i == j; got != want {
				t.Fatalf("%+v.Equal(%+v) = %v, want %v", a, b, got, want)
			}
		}
	}
}
