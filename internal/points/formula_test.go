package points

import (
	"math"
	"testing"
)

func TestFillScoreFiveMinuteExample(t *testing.T) {
	const tAvg, tMin = 300, 120
	cases := []struct {
		t     int
		want  float64
		delta float64
	}{
		{100, 0, 0.01},
		{150, 5.5, 0.15},
		{200, 15.8, 0.15},
		{250, 32.4, 0.15},
		{280, 42.2, 0.15},
		{300, 47.0, 0.15},
		{330, 50, 0.05},
		{350, 49.8, 0.15},
		{400, 48.1, 0.15},
		{450, 44.6, 0.15},
		{600, 27.9, 0.2},
		{700, 16.8, 0.2},
		{950, 2.3, 0.15},
	}
	for _, c := range cases {
		got := FillScore(c.t, tAvg, tMin)
		if math.Abs(got-c.want) > c.delta {
			t.Errorf("t=%d got %.2f want %.1f", c.t, got, c.want)
		}
	}
}
