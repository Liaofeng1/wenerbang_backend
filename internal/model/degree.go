package model

// Academic degree category tags (学科门类), without bachelor/master/phd distinction.
var DegreeTags = []string{
	"哲学",
	"经济学",
	"法学",
	"教育学",
	"文学",
	"历史学",
	"理学",
	"工学",
	"农学",
	"医学",
	"军事学",
	"管理学",
	"艺术学",
}

func IsValidDegreeTag(tag string) bool {
	for _, t := range DegreeTags {
		if t == tag {
			return true
		}
	}
	return false
}

func NormalizeDegreeTags(tags []string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := raw
		if tag == "" {
			continue
		}
		if !IsValidDegreeTag(tag) {
			return nil, false
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out, true
}

// AllowsDegree returns true if survey accepts the filler's degree tag.
// Empty target list means all degrees are allowed.
func (s *Survey) AllowsDegree(degreeTag string) bool {
	if len(s.TargetDegrees) == 0 {
		return true
	}
	for _, t := range s.TargetDegrees {
		if t == degreeTag {
			return true
		}
	}
	return false
}
