package app

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoCookieAllowlistMatchesEngine pins the NoCookie wiring against the
// engine's canonical cookie names. The production call passes
// noCookieAllowlist(); if either side renames a cookie, this fails and forces
// both to move together — silently stripping lesstruct_lang would break
// returning-visitor negotiation with no other test failing.
func TestNoCookieAllowlistMatchesEngine(t *testing.T) {
	allowed := noCookieAllowlist()
	require.Len(t, allowed, 3)
	assert.Contains(t, allowed, sitessr.LangCookieName)
	assert.Contains(t, allowed, sitessr.ConsentCookieName)
	assert.Contains(t, allowed, kernel.SessionCookieName)
}
