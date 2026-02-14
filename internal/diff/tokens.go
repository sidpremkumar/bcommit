package diff

// EstimateTokens returns a conservative token count estimate for the given text.
// Uses len/3 for diff content (many short tokens like +, -, @@, line numbers).
// This avoids needing an external tokenizer — precision isn't critical since
// we're only making tier-routing decisions.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return len(text) / 3
}
