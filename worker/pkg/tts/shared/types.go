package shared

import "context"

type Config struct {
	Provider string

	APIURL string
	APIKey string

	TencentRegion          string
	TencentSecretID        string
	TencentSecretKey       string
	TencentVoiceType       int64
	TencentPrimaryLanguage int64
	TencentModelType       int64
	TencentCodec           string

	ElevenLabsBaseURL      string
	ElevenLabsAPIKey       string
	ElevenLabsVoiceID      string
	ElevenLabsModelID      string
	ElevenLabsOutputFormat string
	ElevenLabsEnableLog    bool
}

type Request struct {
	Text             string
	Language         string
	VoiceType        *int64
	VoiceID          *string
	Speed            *float64
	Stability        *float64
	SimilarityBoost  *float64
	Style            *float64
	UseSpeakerBoost  *bool
	SampleRate       *int64
	EmotionCategory  string
	EmotionIntensity *int64
	EnableSubtitle   *bool
}

type Result struct {
	Audio       []byte
	Ext         string
	Subtitles   []Subtitle
	RawResponse []byte
}

type Subtitle struct {
	Text       string
	BeginTime  int
	EndTime    int
	BeginIndex int
	EndIndex   int
	Phoneme    string
}

type Provider interface {
	Synthesize(ctx context.Context, req Request) (Result, error)
}
