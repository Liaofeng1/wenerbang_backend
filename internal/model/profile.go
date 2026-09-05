package model

var Genders = []string{"男", "女", "不便透露"}

var Regions = []string{"北方", "南方"}

var CityTiers = []string{"一线", "新一线", "二线", "三线", "四线及以下"}

func IsValidGender(v string) bool {
	for _, x := range Genders {
		if x == v {
			return true
		}
	}
	return false
}

func IsValidRegion(v string) bool {
	for _, x := range Regions {
		if x == v {
			return true
		}
	}
	return false
}

func IsValidCityTier(v string) bool {
	for _, x := range CityTiers {
		if x == v {
			return true
		}
	}
	return false
}

func normalizeTags(tags []string, valid func(string) bool) ([]string, bool) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		if raw == "" {
			continue
		}
		if !valid(raw) {
			return nil, false
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out, true
}

func NormalizeGenders(tags []string) ([]string, bool) {
	return normalizeTags(tags, IsValidGender)
}

func NormalizeRegions(tags []string) ([]string, bool) {
	return normalizeTags(tags, IsValidRegion)
}

func NormalizeCityTiers(tags []string) ([]string, bool) {
	return normalizeTags(tags, IsValidCityTier)
}

func allowsAny(targets []string, value string) bool {
	if len(targets) == 0 {
		return true
	}
	for _, t := range targets {
		if t == value {
			return true
		}
	}
	return false
}

func (s *Survey) AllowsGender(gender string) bool {
	return allowsAny(s.TargetGenders, gender)
}

func (s *Survey) AllowsRegion(region string) bool {
	return allowsAny(s.TargetRegions, region)
}

func (s *Survey) AllowsCityTier(tier string) bool {
	return allowsAny(s.TargetCityTiers, tier)
}

func (s *Survey) AllowsUser(u *User) bool {
	if u == nil {
		return false
	}
	return s.AllowsGender(u.Gender) &&
		s.AllowsRegion(u.Region) &&
		s.AllowsCityTier(u.CityTier)
}
