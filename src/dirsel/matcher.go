package dirsel

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"cosmosbdr/src/types"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	SelectionAll     = "all"
	SelectionMatched = "matched"

	PatternGlob  = "glob"
	PatternRegex = "regex"
)

// ValidateScope checks directory selection rules before list/walk.
func ValidateScope(scope *types.DirectoryScope) error {
	if scope == nil {
		return fmt.Errorf("directory scope is required for directory backup")
	}
	sel := strings.ToLower(strings.TrimSpace(scope.Selection))
	if sel == "" {
		sel = SelectionAll
		scope.Selection = SelectionAll
	}
	switch sel {
	case SelectionAll, SelectionMatched:
	default:
		return fmt.Errorf("directory.selection must be %q or %q", SelectionAll, SelectionMatched)
	}
	scope.Selection = sel

	if sel == SelectionAll && len(scope.Include) > 0 {
		return fmt.Errorf("directory.include must be empty when selection is %q", SelectionAll)
	}
	if sel == SelectionMatched && len(scope.Include) == 0 {
		return fmt.Errorf("directory.include is required when selection is %q", SelectionMatched)
	}

	for i, p := range scope.Include {
		if err := validatePattern(p); err != nil {
			return fmt.Errorf("directory.include[%d]: %w", i, err)
		}
	}
	for i, p := range scope.Exclude {
		if err := validatePattern(p); err != nil {
			return fmt.Errorf("directory.exclude[%d]: %w", i, err)
		}
	}
	return nil
}

func validatePattern(p types.PathPattern) error {
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	pat := strings.TrimSpace(p.Pattern)
	switch typ {
	case PatternGlob, PatternRegex:
	default:
		return fmt.Errorf("pattern type must be %q or %q", PatternGlob, PatternRegex)
	}
	if pat == "" {
		return fmt.Errorf("pattern string is required")
	}
	if typ == PatternRegex {
		if _, err := regexp.Compile(pat); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	return nil
}

// Matcher applies locked precedence: depth → selection/include → exclude → size.
type Matcher struct {
	selection string
	globInc   []string
	regexInc  []*regexp.Regexp
	globExc   []string
	regexExc  []*regexp.Regexp
	maxDepth  *int
	minBytes  *int64
	maxBytes  *int64
}

// NewMatcher builds a matcher; scope must already pass ValidateScope.
func NewMatcher(scope types.DirectoryScope) (*Matcher, error) {
	if err := ValidateScope(&scope); err != nil {
		return nil, err
	}
	m := &Matcher{
		selection: scope.Selection,
		maxDepth:  scope.MaxDepth,
		minBytes:  scope.MinFileBytes,
		maxBytes:  scope.MaxFileBytes,
	}
	for _, p := range scope.Include {
		if err := m.addPattern(p, true); err != nil {
			return nil, err
		}
	}
	for _, p := range scope.Exclude {
		if err := m.addPattern(p, false); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Matcher) addPattern(p types.PathPattern, include bool) error {
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	pat := strings.TrimSpace(p.Pattern)
	switch typ {
	case PatternGlob:
		if include {
			m.globInc = append(m.globInc, pat)
		} else {
			m.globExc = append(m.globExc, pat)
		}
	case PatternRegex:
		re, err := regexp.Compile(pat)
		if err != nil {
			return err
		}
		if include {
			m.regexInc = append(m.regexInc, re)
		} else {
			m.regexExc = append(m.regexExc, re)
		}
	}
	return nil
}

// Accept reports whether a file at relToDir (relative to directory root, slash-separated) should be backed up.
func (m *Matcher) Accept(relToDir string, size int64) bool {
	relToDir = normalizeRel(relToDir)
	if relToDir == "" || relToDir == "." {
		return false
	}
	if m.maxDepth != nil {
		depth := depthOf(relToDir)
		if depth > *m.maxDepth {
			return false
		}
	}
	if m.minBytes != nil && size < *m.minBytes {
		return false
	}
	if m.maxBytes != nil && size > *m.maxBytes {
		return false
	}

	if m.selection == SelectionMatched {
		if !m.matchesAny(relToDir, m.globInc, m.regexInc) {
			return false
		}
	}
	if m.matchesAny(relToDir, m.globExc, m.regexExc) {
		return false
	}
	return true
}

func (m *Matcher) matchesAny(rel string, globs []string, regexes []*regexp.Regexp) bool {
	for _, g := range globs {
		if ok, _ := doublestar.Match(g, rel); ok {
			return true
		}
	}
	for _, re := range regexes {
		if re.MatchString(rel) {
			return true
		}
	}
	return false
}

func normalizeRel(rel string) string {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	return path.Clean(rel)
}

// depthOf counts path segments under the directory root (foo = 1, sub/foo.txt = 2).
func depthOf(rel string) int {
	rel = normalizeRel(rel)
	if rel == "" || rel == "." {
		return 0
	}
	return strings.Count(rel, "/") + 1
}
