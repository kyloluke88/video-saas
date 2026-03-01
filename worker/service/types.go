package service

type Config struct {
	URL string

	Host     string
	Port     string
	Username string
	Password string
	VHost    string

	Exchange        string
	ExchangeType    string
	Queue           string
	RoutingKey      string
	RetryQueue      string
	RetryRoutingKey string
	DLX             string
	DLQ             string
	DLQRoutingKey   string
	RetryDelayMs    int
	Prefetch        int
	MaxRetries      int

	SeedanceBaseURL         string
	SeedanceGeneratePath    string
	SeedanceStatusPath      string
	SeedanceAPIKey          string
	SeedancePollIntervalSec int
	SeedanceMaxPollAttempts int
	SeedanceHTTPTimeoutSec  int
	SeedanceDryRunEnable    bool

	FFmpegPostprocessEnabled bool
	BGMEnable                bool
	FFmpegWorkDir            string
	FFmpegTimeoutSec         int

	TTSAPIURL   string
	TTSAPIKey   string
	TTSProvider string

	TTSTencentRegion          string
	TTSTencentSecretID        string
	TTSTencentSecretKey       string
	TTSTencentVoiceType       int64
	TTSTencentPrimaryLanguage int64
	TTSTencentModelType       int64
	TTSTencentCodec           string

	S3Enabled   bool
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3PublicURL string
}

type ProjectPlanPayload struct {
	ProjectID         string
	IdiomName         string
	IdiomNameEn       string
	Dynasty           string
	Platform          string
	Category          string
	NarrationLanguage string
	TargetDurationSec int
	ImageURLs         []string
	Characters        []string
	Props             []string
	SceneElements     []string
	Audience          string
	Tone              string
	AspectRatio       string
	Resolution        string
}

type VisualBible struct {
	StyleAnchor       string `json:"style_anchor,omitempty"`
	CharacterAnchor   string `json:"character_anchor,omitempty"`
	EnvironmentAnchor string `json:"environment_anchor,omitempty"`
	NegativePrompt    string `json:"negative_prompt,omitempty"`
}

type ObjectSpec struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Immutable map[string]interface{} `json:"immutable,omitempty"`
	Mutable   map[string]interface{} `json:"mutable,omitempty"`
}

type ScenePlan struct {
	Index       int                    `json:"index"`
	DurationSec int                    `json:"duration_sec"`
	Goal        string                 `json:"goal,omitempty"`
	ObjectsRef  []string               `json:"objects_ref,omitempty"`
	Composition map[string]interface{} `json:"composition,omitempty"`
	Action      []string               `json:"action,omitempty"`
	Prompt      string                 `json:"prompt"`
	Narration   string                 `json:"narration"`
}

type ProjectPlanResult struct {
	ProjectID         string       `json:"project_id"`
	Platform          string       `json:"platform"`
	Category          string       `json:"category"`
	NarrationLanguage string       `json:"narration_language"`
	TargetDurationSec int          `json:"target_duration_sec"`
	AspectRatio       string       `json:"aspect_ratio"`
	Resolution        string       `json:"resolution"`
	ImageURLs         []string     `json:"image_urls"`
	Characters        []string     `json:"characters,omitempty"`
	Props             []string     `json:"props,omitempty"`
	SceneElements     []string     `json:"scene_elements,omitempty"`
	NarrationFull     string       `json:"narration_full"`
	VisualBible       VisualBible  `json:"visual_bible,omitempty"`
	ObjectRegistry    []ObjectSpec `json:"object_registry,omitempty"`
	Scenes            []ScenePlan  `json:"scenes"`
	CreatedAt         string       `json:"created_at"`
}

type NonRetryableError struct {
	Err error
}

func (e NonRetryableError) Error() string { return e.Err.Error() }
