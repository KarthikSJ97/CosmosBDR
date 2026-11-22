package dirsel

import (
	"testing"

	"cosmosbdr/src/types"
)

func ptrInt(v int) *int { return &v }

func TestMatcher_AllExclude(t *testing.T) {
	scope := types.DirectoryScope{
		Selection: SelectionAll,
		Exclude: []types.PathPattern{
			{Type: "glob", Pattern: "**/*.tmp"},
		},
	}
	m, err := NewMatcher(scope)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Accept("keep.json", 10) {
		t.Fatal("expected keep.json")
	}
	if m.Accept("cache/x.tmp", 10) {
		t.Fatal("expected exclude tmp")
	}
}

func TestMatcher_MatchedInclude(t *testing.T) {
	scope := types.DirectoryScope{
		Selection: SelectionMatched,
		Include: []types.PathPattern{
			{Type: "glob", Pattern: "**/*.json"},
			{Type: "regex", Pattern: `^config/.*\.ya?ml$`},
		},
		Exclude: []types.PathPattern{
			{Type: "glob", Pattern: "**/staging/**"},
		},
	}
	m, err := NewMatcher(scope)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Accept("app/settings.json", 1) {
		t.Fatal("json should match")
	}
	if !m.Accept("config/app.yml", 1) {
		t.Fatal("yaml under config should match regex")
	}
	if m.Accept("readme.txt", 1) {
		t.Fatal("txt should not match include")
	}
	if m.Accept("staging/app.json", 1) {
		t.Fatal("staging should be excluded")
	}
}

func TestMatcher_MaxDepth(t *testing.T) {
	scope := types.DirectoryScope{
		Selection: SelectionAll,
		MaxDepth:  ptrInt(1),
	}
	m, err := NewMatcher(scope)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Accept("top.dat", 1) {
		t.Fatal("depth-1 file at root should match")
	}
	if m.Accept("nested/top.dat", 1) {
		t.Fatal("depth-2 should not match max_depth 1")
	}
}

func TestValidateScope_MatchedRequiresInclude(t *testing.T) {
	err := ValidateScope(&types.DirectoryScope{Selection: SelectionMatched})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateScope_AllRejectsInclude(t *testing.T) {
	err := ValidateScope(&types.DirectoryScope{
		Selection: SelectionAll,
		Include:   []types.PathPattern{{Type: "glob", Pattern: "*.yml"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}