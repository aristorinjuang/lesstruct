package repository

import (
	"strconv"
	"strings"
)

// languageRankExpression renders a CASE expression mapping a language column
// onto its zero-based position in the priority-ordered languages slice. The
// rank integers are generated internally, so they are safe to inline — the
// same reasoning as SortDirectionSQL. Only the language values themselves are
// bound as parameters.
func languageRankExpression(column string, languages []string) (string, []any) {
	var b strings.Builder
	b.WriteString("CASE ")
	b.WriteString(column)
	exprArgs := make([]any, 0, len(languages))
	for i, lang := range languages {
		b.WriteString(" WHEN ? THEN ")
		b.WriteString(strconv.Itoa(i))
		exprArgs = append(exprArgs, lang)
	}
	b.WriteString(" ELSE ")
	b.WriteString(strconv.Itoa(len(languages)))
	b.WriteString(" END")
	return b.String(), exprArgs
}

func sqlPlaceholders(n int) string {
	return strings.TrimRight(strings.Repeat("?, ", n), ", ")
}

// SiblingConjunct is one extra WHERE conjunct applied to the sibling subquery
// of LanguageFilter's fallback predicate. Use it to keep the dedup inside the
// listing's own scope — e.g. the sibling must share the outer query's status,
// post type, author, or tag for it to outrank the candidate row. The SQL must
// reference columns through the sibling alias passed to LanguageFilter.
type SiblingConjunct struct {
	SQL  string
	Args []any
}

// LanguageFilter renders the language restriction for public listings against
// a priority-ordered languages slice, plus the bind args in SQL order. It
// implements Hugo-style language fallback:
//
//   - empty slice: no restriction — every language is returned.
//   - single element: restricted to exactly that language.
//   - multiple elements: each translation group is represented once by its
//     best-ranked available language. A row survives only when no
//     higher-priority published sibling exists in the same translation group
//     (COALESCE(translation_group_id, id) identifies the group).
//
// The predicate is meant to be appended to the outer WHERE clause before
// ORDER BY/LIMIT/OFFSET so pagination stays exact under deduplication.
// alias is the outer content_items table alias and sibling the alias used
// inside the NOT EXISTS subquery; both must differ from every other table
// alias in the query.
func LanguageFilter(alias string, sibling string, languages []string, conjuncts []SiblingConjunct) (string, []any) {
	switch {
	case len(languages) == 0:
		return "", nil
	case len(languages) == 1:
		return " AND " + alias + ".language = ?", []any{languages[0]}
	}

	var b strings.Builder
	args := make([]any, 0, len(languages)*4+len(conjuncts)+2)

	b.WriteString(" AND ")
	b.WriteString(alias)
	b.WriteString(".language IN (")
	b.WriteString(sqlPlaceholders(len(languages)))
	b.WriteString(")")
	for _, lang := range languages {
		args = append(args, lang)
	}

	b.WriteString(" AND NOT EXISTS (SELECT 1 FROM content_items ")
	b.WriteString(sibling)
	b.WriteString(" WHERE COALESCE(")
	b.WriteString(sibling)
	b.WriteString(".translation_group_id, ")
	b.WriteString(sibling)
	b.WriteString(".id) = COALESCE(")
	b.WriteString(alias)
	b.WriteString(".translation_group_id, ")
	b.WriteString(alias)
	b.WriteString(".id)")

	for _, conjunct := range conjuncts {
		b.WriteString(" ")
		b.WriteString(conjunct.SQL)
		args = append(args, conjunct.Args...)
	}

	b.WriteString(" AND ")
	b.WriteString(sibling)
	b.WriteString(".language IN (")
	b.WriteString(sqlPlaceholders(len(languages)))
	b.WriteString(")")
	for _, lang := range languages {
		args = append(args, lang)
	}

	sibExpr, sibArgs := languageRankExpression(sibling+".language", languages)
	aliasExpr, aliasArgs := languageRankExpression(alias+".language", languages)
	b.WriteString(" AND ")
	b.WriteString(sibExpr)
	b.WriteString(" < ")
	b.WriteString(aliasExpr)
	args = append(args, sibArgs...)
	args = append(args, aliasArgs...)

	b.WriteString(")")
	return b.String(), args
}

// DollarPlaceholders rewrites every ? placeholder in fragment into $N
// positional placeholders numbered from start, returning the rewritten
// fragment and the next unused number. PostgreSQL drivers need this because
// LanguageFilter emits portable ? placeholders while the pg queries number
// their bind args; arg order is untouched, so the slice returned by
// LanguageFilter keeps working as-is.
func DollarPlaceholders(fragment string, start int) (string, int) {
	if !strings.Contains(fragment, "?") {
		return fragment, start
	}

	var b strings.Builder
	b.Grow(len(fragment) + 8)
	n := start
	for _, r := range fragment {
		if r == '?' {
			n++
			b.WriteString("$")
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String(), n
}
