package pipeline

import (
	"fmt"
	"strings"
)

func InvalidateOutputs(projectID string, ttsType int, startFrom string) error {
	if _, ok := ParseStage(startFrom); !ok {
		return fmt.Errorf("unsupported practical stage: %s", strings.TrimSpace(startFrom))
	}
	return nil
}

func InvalidateAudioOutputs(projectID string, ttsType int, startFrom string, blockNums, chapterNums []int) error {
	return InvalidateOutputs(projectID, ttsType, startFrom)
}
