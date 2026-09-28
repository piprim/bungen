package util

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fatih/camelcase"
)

const (
	// Id is camelCased Id
	Id = "Id"
	// Ids is camelCased Ids
	Ids = "Ids"
	// ID is golang ID
	ID = "ID"
	// IDs is golang IDs
	IDs = "IDs"
	// Rel if suffix for Relation
	Rel = "Rel"
)

// IsUpper check rune for upper case
func IsUpper(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

// IsLower check rune for lower case
func IsLower(c byte) bool {
	return c >= 'a' && c <= 'z'
}

// ToUpper converts rune to upper
func ToUpper(c byte) byte {
	return c - 32
}

// ToLower converts rune to lower
func ToLower(c byte) byte {
	return c + 32
}

// CamelCased converts string to camelCase
// from https://github.com/uptrace/bun/blob/master/internal/underscore.go#L37
func CamelCased(s string) string {
	r := make([]byte, 0, len(s))
	upperNext := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			upperNext = true
			continue
		}
		if upperNext {
			if IsLower(c) {
				c = ToUpper(c)
			}
			upperNext = false
		}
		r = append(r, c)
	}
	return string(r)
}

// Underscore converts string to under_scored
// from https://github.com/uptrace/bun/blob/master/internal/underscore.go#L19
func Underscore(s string) string {
	r := make([]byte, 0, len(s)+5)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if IsUpper(c) {
			if i > 0 && i+1 < len(s) && (IsLower(s[i-1]) || IsLower(s[i+1])) {
				r = append(r, '_', ToLower(c))
			} else {
				r = append(r, ToLower(c))
			}
		} else {
			r = append(r, c)
		}
	}
	return string(r)
}

// Sanitize makes string suitable for golang var, const, field, type name
func Sanitize(s string) string {
	rgxp := regexp.MustCompile(`[^a-zA-Z\d\-_]`)
	sanitized := strings.Replace(rgxp.ReplaceAllString(s, ""), "-", "_", -1)

	if len(sanitized) != 0 && ((sanitized[0] >= '0' && sanitized[0] <= '9') || sanitized[0] == '_') {
		sanitized = "T" + sanitized
	}

	return sanitized
}

// PackageName gets string usable as package name
func PackageName(s string) string {
	return strings.ToLower(Sanitize(s))
}

// PackageFromOutput derives a package name from the folder holding the output file.
// A bare file name resolves to the current working directory. When no usable
// name can be derived (for example the file system root), DefaultPackage is used.
func PackageFromOutput(output string) string {
	abs, err := filepath.Abs(output)
	if err != nil {
		return DefaultPackage
	}

	pkg := PackageName(filepath.Base(filepath.Dir(abs)))
	if pkg == "" {
		return DefaultPackage
	}

	return pkg
}

// EntityName gets string usable as struct name
func EntityName(s string) string {
	return strings.Join(camelcase.Split(CamelCased(Sanitize(s))), "")
}

// ColumnName gets string usable as struct field name
func ColumnName(s string) string {
	camelCased := CamelCased(Sanitize(s))
	camelCased = ReplaceSuffix(ReplaceSuffix(camelCased, Id, ID), Ids, IDs)

	return strings.Title(camelCased)
}

// HasUpper checks if string contains upper case
func HasUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if IsUpper(c) {
			return true
		}
	}
	return false
}

// ReplaceSuffix replaces substring on the end of string
func ReplaceSuffix(in, suffix, replace string) string {
	if strings.HasSuffix(in, suffix) {
		in = in[:len(in)-len(suffix)] + replace
	}
	return in
}

// LowerFirst lowers the first letter
func LowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
