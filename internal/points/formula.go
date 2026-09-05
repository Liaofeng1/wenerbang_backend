package points

import "math"

// Nonprofit fill-reward curve from 问而帮非盈利版 §3.2.
// P = 10 * t_avg / 60; peak sits 30s after the current average.
const (
	PointsPerMinute = 10.0
	FastVariance    = 14688.0
	SlowVariance    = 125000.0
	PeakLeadSeconds = 30.0
	DefaultTmin     = 120
	DefaultTavg     = 300
)

func PeakScore(tAvg int) float64 {
	if tAvg < 0 {
		tAvg = 0
	}
	return PointsPerMinute * float64(tAvg) / 60.0
}

func PeakReward(tAvg int) int {
	return int(math.Round(PeakScore(tAvg)))
}

// FillReward returns integer points for stay time t given the running average tAvg and publisher Tmin.
func FillReward(t, tAvg, tMin int) int {
	return int(math.Round(FillScore(t, tAvg, tMin)))
}

func FillScore(t, tAvg, tMin int) float64 {
	if tMin < 0 {
		tMin = 0
	}
	if t < tMin {
		return 0
	}
	if tAvg < tMin {
		tAvg = tMin
	}
	p := PeakScore(tAvg)
	peak := float64(tAvg) + PeakLeadSeconds
	dt := float64(t) - peak
	if dt < 0 {
		return p * math.Exp(-(dt*dt)/FastVariance)
	}
	return p * math.Exp(-(dt*dt)/SlowVariance)
}
