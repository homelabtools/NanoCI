package regex

import (
	"regexp"
)

// Capture1 returns exactly 1 captured string from a regex with 1 capture group, or false if there are no matches
func Capture1(regex *regexp.Regexp, str string) (string, bool) {
	if m, ok := CaptureN(regex, str, 1); ok {
		return m[0], true
	}
	return "", false
}

// Capture2 returns exactly 2 captured string from a regex with 2 capture group, or false if there are no matches
func Capture2(regex *regexp.Regexp, str string) (string, string, bool) {
	if m, ok := CaptureN(regex, str, 2); ok {
		return m[0], m[1], true
	}
	return "", "", false
}

// Capture3 returns exactly 3 captured string from a regex with 3 capture group, or false if there are no matches
func Capture3(regex *regexp.Regexp, str string) (string, string, string, bool) {
	if m, ok := CaptureN(regex, str, 3); ok {
		return m[0], m[1], m[2], true
	}
	return "", "", "", false
}

// CaptureN returns exactly N captured string from a regex with N capture group, or false if there are no matches
func CaptureN(regex *regexp.Regexp, str string, n int) ([]string, bool) {
	matches := regex.FindAllStringSubmatch(str, -1)
	if len(matches) != 1 {
		return nil, false
	}
	if len(matches[0]) != n+1 {
		return nil, false
	}
	return matches[0][1:], true
}
