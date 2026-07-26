package chat

import (
	"math"
	"testing"
)

// Curves below are the literal five-point probe readings from the plugins, so
// this test fails if the inversion stops matching what a real plugin does.
func TestEQNormalizedFromMeasuredCurves(t *testing.T) {
	cases := []struct {
		name   string
		curve  [][2]float64
		target float64
		// wantNormalized is derived from the plugin's own response, not from the
		// inversion under test: it is the n that the curve's closed form maps to
		// target.
		wantNormalized float64
	}{
		{
			// v = 10 · 4000^n
			name:           "TDR Nova frequency (logarithmic)",
			curve:          [][2]float64{{0, 10}, {0.25, 80}, {0.5, 632}, {0.75, 5000}, {1, 40000}},
			target:         3400,
			wantNormalized: math.Log(340) / math.Log(4000),
		},
		{
			// v = 20 + 19980·n²  — declares no unit at all
			name:           "FreeEQ8 frequency (quadratic)",
			curve:          [][2]float64{{0, 20}, {0.25, 1268.750}, {0.5, 5015}, {0.75, 11258.751}, {1, 20000}},
			target:         3400,
			wantNormalized: math.Sqrt(3380.0 / 19980.0),
		},
		{
			// v = 600 + 6400·n — declares "Hz", which would imply logarithmic
			name:           "ZamEQ2 frequency 2 (linear)",
			curve:          [][2]float64{{0, 600}, {0.25, 2200}, {0.5, 3800}, {0.75, 5400}, {1, 7000}},
			target:         3400,
			wantNormalized: 2800.0 / 6400.0,
		},
		{
			// v = 10 · 3000^n
			name:           "Pro-Q 3 frequency (logarithmic)",
			curve:          [][2]float64{{0, 10}, {0.25, 74.008}, {0.5, 547.72}, {0.75, 4053.6}, {1, 30000}},
			target:         3400,
			wantNormalized: math.Log(340) / math.Log(3000),
		},
		{
			name:           "TDR Nova gain (linear through zero)",
			curve:          [][2]float64{{0, -18}, {0.25, -9}, {0.5, 0}, {0.75, 9}, {1, 18}},
			target:         -3,
			wantNormalized: 15.0 / 36.0,
		},
		{
			// v = 0.1 + 23.9·n²
			name:           "FreeEQ8 Q (quadratic)",
			curve:          [][2]float64{{0, 0.1}, {0.25, 1.594}, {0.5, 6.075}, {0.75, 13.544}, {1, 24}},
			target:         6.075,
			wantNormalized: 0.5,
		},
		{
			// v = 0.1 · 60^n
			name:           "TDR Nova Q (logarithmic)",
			curve:          [][2]float64{{0, 0.10}, {0.25, 0.28}, {0.5, 0.77}, {0.75, 2.16}, {1, 6.00}},
			target:         0.77,
			wantNormalized: 0.5,
		},
	}
	for _, tc := range cases {
		got, ok := eqNormalizedFromCurve(tc.target, tc.curve)
		if !ok {
			t.Errorf("%s: inversion refused a strictly increasing curve", tc.name)
			continue
		}
		if math.Abs(got-tc.wantNormalized) > 0.01 {
			t.Errorf("%s: normalized = %.5f, want %.5f (a %.1f%% position error)",
				tc.name, got, tc.wantNormalized, math.Abs(got-tc.wantNormalized)*100)
		}
	}
}

func TestEQNormalizedFromCurveClampsAndRefuses(t *testing.T) {
	curve := [][2]float64{{0, 20}, {0.5, 5015}, {1, 20000}}
	if got, ok := eqNormalizedFromCurve(5, curve); !ok || got != 0 {
		t.Errorf("below-range target = %v, %v; want 0, true", got, ok)
	}
	if got, ok := eqNormalizedFromCurve(99999, curve); !ok || got != 1 {
		t.Errorf("above-range target = %v, %v; want 1, true", got, ok)
	}
	// Too few points, and non-monotonic input, must fall back rather than guess.
	if _, ok := eqNormalizedFromCurve(100, [][2]float64{{0, 20}, {1, 20000}}); ok {
		t.Error("a two-point curve must be refused")
	}
}

// A plugin whose response matches neither family must still produce a usable
// answer rather than a wild one: the better-fitting of the two is used, and the
// result has to stay inside the bracketing samples.
func TestEQNormalizedFromCurveStaysBracketedOnAnOddCurve(t *testing.T) {
	curve := [][2]float64{{0, 100}, {0.25, 900}, {0.5, 1000}, {0.75, 1100}, {1, 5000}}
	got, ok := eqNormalizedFromCurve(1000, curve)
	if !ok {
		t.Fatal("expected an answer for a strictly increasing curve")
	}
	if got < 0.25 || got > 0.75 {
		t.Errorf("normalized = %.3f for the midpoint sample, want within [0.25, 0.75]", got)
	}
}
