package model

import "strings"

// MatchPattern reports whether path matches a pattern. A pattern matches the
// path itself, and a pattern that ends in "/..." also matches every path
// below its base. A match never extends a path segment, so "k8s.io/api" does
// not match "k8s.io/apiserver".
func MatchPattern(pattern, path string) bool {
	base, subtree := strings.CutSuffix(pattern, "/...")
	if !subtree {
		return path == pattern
	}
	return Within(path, base)
}

// MatchExternal reports whether an import of path, from the module with path
// mod, matches the external pattern. An external pattern never crosses into
// another module: a pattern that ends in "/..." matches only the packages of
// the module that contains its base, so a nested module or another major
// version needs its own entry.
func MatchExternal(pattern, path, mod string) bool {
	if !MatchPattern(pattern, path) {
		return false
	}
	base, subtree := strings.CutSuffix(pattern, "/...")
	return !subtree || Within(base, mod)
}

// Within reports whether the slash-separated path is root or a path below it.
func Within(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

// IsStandard reports whether an import path can name a package of the
// standard library: the first element of every other import path contains a
// dot.
func IsStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
