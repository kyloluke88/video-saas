package podcast_audio_service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
)

const youtubeTranscriptFilename = "youtube_transcript.srt"

func exportYouTubeTranscriptFile(projectDir string, script dto.PodcastScript) error {
	content := buildYouTubeTranscriptSRTWithLeadIn(script, youtubePublishLeadInMS(script.Language))
	if strings.TrimSpace(content) == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(projectDir, youtubeTranscriptFilename), []byte(content), 0o644)
}

func buildYouTubeTranscriptSRT(script dto.PodcastScript) string {
	return buildYouTubeTranscriptSRTWithLeadIn(script, 0)
}

func buildYouTubeTranscriptSRTWithLeadIn(script dto.PodcastScript, leadInMS int) string {
	segments := transcriptSegments(script)
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	index := 1
	for _, seg := range segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		text := transcriptCueText(script.Language, seg)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"%d\n%s --> %s\n%s\n\n",
			index,
			formatSRTTimestampMS(seg.StartMS+leadInMS),
			formatSRTTimestampMS(seg.EndMS+leadInMS),
			text,
		))
		index++
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func transcriptSegments(script dto.PodcastScript) []dto.PodcastSegment {
	if len(script.Segments) == 0 && len(script.Blocks) > 0 {
		script.RefreshSegmentsFromBlocks()
	}
	if len(script.Segments) == 0 {
		return nil
	}
	out := make([]dto.PodcastSegment, 0, len(script.Segments))
	for _, seg := range script.Segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		if strings.TrimSpace(transcriptCueText(script.Language, seg)) == "" {
			continue
		}
		out = append(out, seg)
	}
	return out
}

func transcriptCueText(language string, seg dto.PodcastSegment) string {
	text := exportDisplayText(language, seg)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func formatSRTTimestampMS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	ms -= hours * 3600000
	minutes := ms / 60000
	ms -= minutes * 60000
	seconds := ms / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}
