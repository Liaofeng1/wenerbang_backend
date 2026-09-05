package level

import (
	"time"

	"wenbang/internal/model"
)

// XP thresholds (cumulative) from 问而帮 §3.3.5.
// Index i-1 = min exp for level i.
var Thresholds = []int{0, 15, 120, 280, 480, 720, 1000}

var Titles = []string{
	"",
	"问卷萌新",
	"初级填卷人",
	"活跃问卷侠",
	"问卷达人",
	"问卷大师",
	"问卷传奇",
	"问而帮·真神",
}

var PinDiscountPct = []int{0, 100, 100, 90, 80, 70, 60, 50}

var TargetingDiscountPct = []int{0, 100, 100, 100, 95, 90, 85, 80}

var FreePinPerMonth = []int{0, 0, 0, 0, 1, 2, 3, 5}

const (
	XPCheckIn          = 5
	XPFill             = 10
	XPPublish          = 30
	DailyCheckInPoints = 10 // 每日签到固定赠送积分（与经验分开）
)

func clampLv(lv int) int {
	if lv < 1 {
		return 1
	}
	if lv > 7 {
		return 7
	}
	return lv
}

func LevelOf(exp int) int {
	if exp < 0 {
		exp = 0
	}
	lv := 1
	for i := 1; i <= 7; i++ {
		if exp >= Thresholds[i-1] {
			lv = i
		}
	}
	return lv
}

func TitleOf(lv int) string {
	return Titles[clampLv(lv)]
}

func Progress(exp int) (lv int, title string, next int, pct float64, atMax bool, toNext int) {
	lv = LevelOf(exp)
	title = TitleOf(lv)
	if lv >= 7 {
		return lv, title, Thresholds[6], 100, true, 0
	}
	cur := Thresholds[lv-1]
	next = Thresholds[lv]
	span := next - cur
	got := exp - cur
	if got < 0 {
		got = 0
	}
	pct = float64(got) * 100 / float64(span)
	if pct > 100 {
		pct = 100
	}
	toNext = next - exp
	if toNext < 0 {
		toNext = 0
	}
	return lv, title, next, pct, false, toNext
}

func CheckInPointReward(_ int) int {
	return DailyCheckInPoints
}

func ApplyPinDiscount(baseCost, lv int) int {
	if baseCost <= 0 {
		return 0
	}
	return (baseCost * PinDiscountPct[clampLv(lv)]) / 100
}

func ApplyTargetingDiscount(baseCost, lv int) int {
	if baseCost <= 0 {
		return 0
	}
	return (baseCost * TargetingDiscountPct[clampLv(lv)]) / 100
}

func FreePinsAllowed(lv int) int {
	return FreePinPerMonth[clampLv(lv)]
}

func Today() string {
	return time.Now().Format("2006-01-02")
}

func MonthKey() string {
	return time.Now().Format("2006-01")
}

// FillUser populates computed growth fields on the user for API responses.
func FillUser(u *model.User) {
	if u == nil {
		return
	}
	lv, title, next, pct, atMax, toNext := Progress(u.Exp)
	u.Level = lv
	u.LevelTitle = title
	u.NextLevelExp = next
	u.ExpToNext = toNext
	u.LevelProgressPct = pct
	u.LevelAtMax = atMax
	u.CheckedInToday = u.LastCheckIn == Today()
	u.PinDiscountPct = PinDiscountPct[lv]
	u.TargetDiscountPct = TargetingDiscountPct[lv]

	month := MonthKey()
	used := u.FreePinUsed
	if u.FreePinMonth != month {
		used = 0
	}
	remain := FreePinsAllowed(lv) - used
	if remain < 0 {
		remain = 0
	}
	u.FreePinRemain = remain
}

func AddExp(u *model.User, delta int) {
	if u == nil || delta <= 0 {
		return
	}
	u.Exp += delta
}
