package task

import (
	"reflect"
	"testing"
)

func TestBuildTaskMaterialsPodcastStages(t *testing.T) {
	cases := []struct {
		name     string
		taskType string
		payload  map[string]interface{}
		want     []string
	}{
		{
			name:     "upload",
			taskType: "upload.v1",
			want:     []string{"chat_script.pdf"},
		},
		{
			name:     "audio-generate",
			taskType: "podcast.audio.generate.v1",
			payload: map[string]interface{}{
				"script_filename": "scripts/ja_script.json",
			},
			want: []string{"ja_script.json"},
		},
		{
			name:     "audio-align",
			taskType: "podcast.audio.align.v1",
			want:     []string{"script_input.json", "blocks", "block_states"},
		},
		{
			name:     "compose-render",
			taskType: "podcast.compose.render.v1",
			payload: map[string]interface{}{
				"lang":             "ja",
				"bg_img_filenames": []interface{}{"assets/podcast/bg-images/ja7.png"},
			},
			want: []string{"ja7.png", "headphone.gif", "ja_logo.png", "dialogue.mp3"},
		},
		{
			name:     "compose-finalize",
			taskType: "podcast.compose.finalize.v1",
			want:     []string{"podcast_base.mp4", "script_aligned.json", "dialogue.mp3"},
		},
		{
			name:     "page-persist",
			taskType: "podcast.page.persist.v1",
			want:     []string{"request_payload.json", "script_aligned.json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTaskMaterials(VideoTaskMessage{
				TaskType: tc.taskType,
				Payload:  tc.payload,
			})
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected materials\nwant: %v\ngot:  %v", tc.want, got)
			}
		})
	}
}
