package storage

import "strings"

// compareSemver orders two SemVer 2.0.0 strings: -1 when a < b, 0 when they
// have equal precedence, 1 when a > b. Build metadata is ignored and a
// pre-release sorts below its release. Strings that are not valid versions
// sort after valid ones and fall back to byte order among themselves.
func compareSemver(a, b string) int {
	pa, okA := parseSemver(a)
	pb, okB := parseSemver(b)
	switch {
	case !okA && !okB:
		return strings.Compare(a, b)
	case !okA:
		return 1
	case !okB:
		return -1
	}
	for i := 0; i < 3; i++ {
		if c := compareNumeric(pa.core[i], pb.core[i]); c != 0 {
			return c
		}
	}
	return comparePrerelease(pa.pre, pb.pre)
}

type semver struct {
	core [3]string
	pre  []string
}

func parseSemver(v string) (semver, bool) {
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var out semver
	if i := strings.IndexByte(v, '-'); i >= 0 {
		if i == len(v)-1 {
			return semver{}, false
		}
		out.pre = strings.Split(v[i+1:], ".")
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	for i, p := range parts {
		if !isNumeric(p) {
			return semver{}, false
		}
		out.core[i] = p
	}
	return out, true
}

func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		an, bn := isNumeric(a[i]), isNumeric(b[i])
		var c int
		switch {
		case an && bn:
			c = compareNumeric(a[i], b[i])
		case an:
			c = -1
		case bn:
			c = 1
		default:
			c = strings.Compare(a[i], b[i])
		}
		if c != 0 {
			return c
		}
	}
	return compareInt(len(a), len(b))
}

// compareNumeric compares non-negative decimal strings without overflow by
// ignoring leading zeros and comparing length first.
func compareNumeric(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if c := compareInt(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	return strings.Trim(s, "0123456789") == ""
}
