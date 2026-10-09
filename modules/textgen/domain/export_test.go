package textgen

// BuildHTMLSystemPromptForTest exports buildHTMLSystemPrompt for testing.
func BuildHTMLSystemPromptForTest(themeBaseCSS, themeStyleCSS string) string {
	return buildHTMLSystemPrompt(themeBaseCSS, themeStyleCSS)
}

// ParseEnhanceEnvelopeForTest exports parseEnhanceEnvelope for testing.
func ParseEnhanceEnvelopeForTest(response string) (EnhanceResult, error) {
	return parseEnhanceEnvelope(response)
}

// ParseMetadataEnvelopeForTest exports parseMetadataEnvelope for testing.
func ParseMetadataEnvelopeForTest(responseText, format string) (string, string, string, error) {
	return parseMetadataEnvelope(responseText, format)
}

// StripJSONFencesForTest exports stripJSONFences for testing.
func StripJSONFencesForTest(response string) string {
	return stripJSONFences(response)
}
