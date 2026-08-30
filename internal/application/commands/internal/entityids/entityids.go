// Package entityids owns the shape of an entity id: the type's prefix, a
// hyphen, and a decimal suffix with no sign, no padding and no separators.
//
// add allocates ids, update and validate judge them, and add also reads the
// existing ones back to find the next free suffix. Those are the same rule
// read from three sides: an id add can produce but validate calls malformed
// would make a fresh workspace non-conforming, and an id add fails to
// recognise on re-read would be handed out twice.
//
// A suffix that does not fit in an int is refused rather than wrapped. The
// number is used to allocate the next id, and a wrapped one allocates over
// something that already exists.
package entityids

import (
	"strconv"
	"strings"
)

// Format renders the id for a type prefix and suffix.
func Format(prefix string, suffix int) string {
	return prefix + "-" + strconv.Itoa(suffix)
}

// ParseSuffix reports the numeric suffix of an id issued for the given prefix.
// It returns false when the id belongs to another prefix or is not of the
// form the type declares, which is the only signal callers need to reject it.
func ParseSuffix(id string, prefix string) (int, bool) {
	expectedPrefix := prefix + "-"
	if !strings.HasPrefix(id, expectedPrefix) {
		return 0, false
	}

	rawSuffix := strings.TrimPrefix(id, expectedPrefix)
	if rawSuffix == "" {
		return 0, false
	}
	for _, ch := range rawSuffix {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}

	suffix, err := strconv.Atoi(rawSuffix)
	if err != nil || suffix < 0 {
		return 0, false
	}
	return suffix, true
}
