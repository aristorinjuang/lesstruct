package repository_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageFilter(t *testing.T) {
	tests := []struct {
		name          string
		alias         string
		sibling       string
		languages     []string
		conjuncts     []repository.SiblingConjunct
		wantFragment  string
		wantArgs      []any
		fragmentParts []string
		wantArgCount  int
	}{
		{
			name:         "empty languages yields no restriction",
			alias:        "c",
			sibling:      "lang_sibling",
			languages:    nil,
			wantFragment: "",
			wantArgs:     nil,
		},
		{
			name:         "single language restricts exactly",
			alias:        "c",
			sibling:      "lang_sibling",
			languages:    []string{"en"},
			wantFragment: " AND c.language = ?",
			wantArgs:     []any{"en"},
		},
		{
			name:      "multiple languages deduplicate translation groups",
			alias:     "c",
			sibling:   "lang_sibling",
			languages: []string{"en", "id"},
			conjuncts: []repository.SiblingConjunct{
				{SQL: "AND lang_sibling.status = ?", Args: []any{"published"}},
			},
			wantArgCount: 9,
			fragmentParts: []string{
				" AND c.language IN (?, ?)",
				" AND NOT EXISTS (SELECT 1 FROM content_items lang_sibling",
				"WHERE COALESCE(lang_sibling.translation_group_id, lang_sibling.id) = COALESCE(c.translation_group_id, c.id)",
				" AND lang_sibling.status = ?",
				" AND lang_sibling.language IN (?, ?)",
				"CASE lang_sibling.language WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END",
				"CASE c.language WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END",
				" < ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragment, args := repository.LanguageFilter(tt.alias, tt.sibling, tt.languages, tt.conjuncts)

			if tt.wantFragment != "" || tt.languages == nil {
				assert.Equal(t, tt.wantFragment, fragment)
			}
			if tt.wantArgs != nil {
				assert.Equal(t, tt.wantArgs, args)
			}
			for _, part := range tt.fragmentParts {
				assert.Contains(t, fragment, part)
			}
			if tt.wantArgCount != 0 {
				require.Len(t, args, tt.wantArgCount)
			}
		})
	}
}

func TestLanguageFilter_MultiArgumentOrder(t *testing.T) {
	_, args := repository.LanguageFilter("c", "lang_sibling", []string{"en", "id"}, []repository.SiblingConjunct{
		{SQL: "AND lang_sibling.status = ?", Args: []any{"published"}},
	})

	expected := []any{
		"en", "id",
		"published",
		"en", "id",
		"en", "id",
		"en", "id",
	}
	assert.Equal(t, expected, args)
	assert.Equal(t, 9, len(args))
}

func TestDollarPlaceholders(t *testing.T) {
	tests := []struct {
		name       string
		fragment   string
		start      int
		wantString string
		wantNext   int
	}{
		{
			name:       "no placeholders returns input unchanged",
			fragment:   " AND c.status = 1",
			start:      3,
			wantString: " AND c.status = 1",
			wantNext:   3,
		},
		{
			name:       "renumbers placeholders sequentially",
			fragment:   " AND c.language IN (?, ?)",
			start:      2,
			wantString: " AND c.language IN ($3, $4)",
			wantNext:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, next := repository.DollarPlaceholders(tt.fragment, tt.start)
			assert.Equal(t, tt.wantString, got)
			assert.Equal(t, tt.wantNext, next)
		})
	}
}
