package utils

// EstimateTokenCount provides a simple token count estimation.
// Phase 1: simple character-based estimation (Chinese ~0.5 tokens/char, English ~0.25 tokens/char).
// Added buffer accounts for response and overhead.
//
// Phase 2 (future): integrate tiktoken-go for model-specific accurate tokenization.
const (
	// ResponseBufferTokens is the estimated token overhead for LLM response
	ResponseBufferTokens = 200
	// MinEstimatedTokens is the floor for estimation to avoid zero
	MinEstimatedTokens = 50
)

// EstimateTokens returns an estimated token count for the given text.
// Uses a simple heuristic: len(text)/4 ≈ tokens for mixed Chinese/English text.
// Always returns at least MinEstimatedTokens.
func EstimateTokens(text string) int {
	n := len(text) / 4 // rune count would be more accurate for CJK, but byte len/4 is a reasonable approximation
	if n < MinEstimatedTokens {
		return MinEstimatedTokens
	}
	return n
}

// EstimateTokensWithBuffer returns EstimateTokens(text) + ResponseBufferTokens.
func EstimateTokensWithBuffer(text string) int {
	return EstimateTokens(text) + ResponseBufferTokens
}
