package textgen

// BuildHTMLSystemPromptForTest exports buildHTMLSystemPrompt for testing.
func BuildHTMLSystemPromptForTest(themeBaseCSS, themeStyleCSS string) string {
	return buildHTMLSystemPrompt(themeBaseCSS, themeStyleCSS)
}
