package assets

import "strings"

// Match reports whether the project-relative path name matches the glob
// pattern. Both sides tolerate Windows separators (backslashes are normalized
// to forward slashes) and matching is case-insensitive, mirroring the
// case-insensitive filesystems rotor projects usually live on.
//
// Pattern syntax (the doublestar subset rotor needs; stdlib filepath.Glob has
// no `**`, so this is implemented here — no dependencies):
//
//	"*"  matches any run of characters within a single path segment
//	"?"  matches exactly one character within a segment
//	"**" matches any number of whole segments, including zero (only when it
//	     is an entire segment; `a**b` degrades to `*` semantics)
func Match(pattern, name string) bool {
	pat := strings.ToLower(strings.ReplaceAll(pattern, "\\", "/"))
	nm := strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	return matchSegments(splitClean(pat), splitClean(nm))
}

// splitClean splits a slash path into segments, dropping empty segments and
// "." so "./assets//x.png" and "assets/x.png" compare equal.
func splitClean(p string) []string {
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, s := range parts {
		if s != "" && s != "." {
			out = append(out, s)
		}
	}
	return out
}

// matchSegments matches pattern segments against name segments, recursing
// only for `**` (which may swallow zero or more whole segments).
func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(name); i++ {
				if matchSegments(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 || !matchSegment(pat[0], name[0]) {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// matchSegment matches one pattern segment against one name segment with the
// classic iterative star-backtracking algorithm (`*` and `?` only).
func matchSegment(pat, s string) bool {
	pr, sr := []rune(pat), []rune(s)
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(sr) {
		switch {
		case pi < len(pr) && (pr[pi] == '?' || pr[pi] == sr[si]):
			pi++
			si++
		case pi < len(pr) && pr[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(pr) && pr[pi] == '*' {
		pi++
	}
	return pi == len(pr)
}

// staticPrefix returns the leading wildcard-free segments of a (slash,
// cleaned) pattern, used as the walk root so scanning `assets/**/*.png`
// never touches the rest of the project tree.
func staticPrefix(pattern string) string {
	segs := splitClean(pattern)
	var keep []string
	for _, s := range segs {
		if strings.ContainsAny(s, "*?") {
			break
		}
		keep = append(keep, s)
	}
	return strings.Join(keep, "/")
}
