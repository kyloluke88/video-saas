package podcast_audio_service

import (
	"strings"

	"worker/internal/dto"
)

func sanitizeScriptTokens(script dto.PodcastScript) dto.PodcastScript {
	for i := range script.Blocks {
		for j := range script.Blocks[i].Segments {
			script.Blocks[i].Segments[j] = sanitizeSegmentTokens(script.Blocks[i].Segments[j])
		}
	}
	script.RefreshSegmentsFromBlocks()
	return script
}

func sanitizeSegmentTokens(seg dto.PodcastSegment) dto.PodcastSegment {
	if len(seg.Tokens) == 0 {
		return seg
	}
	tokens := make([]dto.PodcastToken, 0, len(seg.Tokens))
	for _, token := range seg.Tokens {
		rawChar := token.Char
		char := strings.TrimSpace(rawChar)
		reading := strings.TrimSpace(token.Reading)

		// Preserve whitespace tokens as explicit word boundaries for inline
		// English display. They should remain visually silent, but they stop
		// adjacent latin tokens from collapsing into one long word.
		if char == "" && strings.TrimSpace(rawChar) == "" && rawChar != "" {
			token.Char = " "
			token.Reading = ""
			tokens = append(tokens, token)
			continue
		}

		// Fully empty placeholder tokens are useless noise from the LLM, so we
		// can safely drop them instead of trying to guess the missing character.
		if char == "" && reading == "" {
			continue
		}

		token.Char = char
		token.Reading = reading
		tokens = append(tokens, token)
	}
	seg.Tokens = tokens
	return seg
}
