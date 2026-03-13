package seedance

type Config struct {
	BaseURL         string
	GeneratePath    string
	StatusPath      string
	APIKey          string
	PollIntervalSec int
	MaxPollAttempts int
	HTTPTimeoutSec  int
}

type GenerateRequest struct {
	Prompt         string `json:"prompt"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	Duration       string `json:"duration,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	GenerateAudio  bool   `json:"generate_audio"`
	FixedLens      bool   `json:"fixed_lens"`
}

type GenerateResult struct {
	VideoURL           string
	CreateResponseBody []byte
	StatusResponseBody []byte
}

type generateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	} `json:"data"`
}

type statusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID   string   `json:"task_id"`
		Status   string   `json:"status"`
		Response []string `json:"response"`
		VideoURL string   `json:"video_url"`
		Error    string   `json:"error"`
		Output   []struct {
			URL string `json:"url"`
		} `json:"output"`
	} `json:"data"`
}

type NonRetryableError struct {
	Err error
}

func (e NonRetryableError) Error() string { return e.Err.Error() }
