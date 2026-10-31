package infrastructure

// BuildHTMLSystemPromptForTest exports buildHTMLSystemPrompt for testing.
func BuildHTMLSystemPromptForTest(themeBaseCSS, themeStyleCSS string) string {
	return buildHTMLSystemPrompt(themeBaseCSS, themeStyleCSS)
}

// StripJSONFencesForTest exports stripJSONFences for testing.
func StripJSONFencesForTest(response string) string {
	return stripJSONFences(response)
}
