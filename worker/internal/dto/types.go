package dto

type VideoTaskMessage struct {
	TaskID    string                 `json:"task_id"`
	TaskType  string                 `json:"task_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt string                 `json:"created_at"`
}

type VisualBible struct {
	StyleAnchor    string `json:"style_anchor,omitempty"`
	ColorPalette   string `json:"color_palette,omitempty"`
	Lighting       string `json:"lighting,omitempty"`
	EraSetting     string `json:"era_setting,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

type ObjectSpec struct {
	ID        string                 `json:"id"`
	Immutable map[string]interface{} `json:"immutable,omitempty"`
}

type CameraSpec struct {
	TRange        string `json:"t_range"`
	ShotType      string `json:"shot_type"`
	Angle         string `json:"angle"`
	Movement      string `json:"movement"`
	MovementSpeed string `json:"movement_speed"`
	Focus         string `json:"focus"`
	Composition   string `json:"composition"`
}

type Beat struct {
	TRange        string `json:"t_range"`
	MicroAction   string `json:"micro_action"`
	VisibleChange string `json:"visible_change"`
	EmotionalTone string `json:"emotional_tone"`
}

type ActionUnit struct {
	UnitGoal   string `json:"unit_goal"`
	StartState string `json:"start_state"`
	Beats      []Beat `json:"beats"`
	EndState   string `json:"end_state"`
}

type Dialogue struct {
	Speaker string `json:"speaker"`
	TRange  string `json:"t_range"`
	Content string `json:"content"`
}

type SoundEffect struct {
	TRange      string `json:"t_range"`
	Description string `json:"description"`
}

type AudioLayer struct {
	Dialogue     []Dialogue    `json:"dialogue"`
	SoundEffects []SoundEffect `json:"sound_effects"`
	Ambience     string        `json:"ambience"`
}

type Cast struct {
	Characters  []string `json:"characters"`
	Props       []string `json:"props"`
	Environment string   `json:"environment"`
}

type ScenePlan struct {
	SceneID          int          `json:"scene_id"`
	DurationSeconds  int          `json:"duration_seconds"`
	ActionUnit       ActionUnit   `json:"action_unit"`
	AudioLayer       AudioLayer   `json:"audio_layer"`
	Camera           []CameraSpec `json:"camera"`
	Cast             Cast         `json:"cast"`
	TransitionToNext string       `json:"transition_to_next"`
}

type ProjectMeta struct {
	Title              string `json:"title"`
	TargetTotalSeconds int    `json:"target_total_seconds"`
}

type ObjectRegistry struct {
	Characters   []ObjectSpec `json:"characters,omitempty"`
	Props        []ObjectSpec `json:"props,omitempty"`
	Environments []ObjectSpec `json:"environments,omitempty"`
}

type ProjectPlanResult struct {
	Meta           ProjectMeta    `json:"meta"`
	VisualBible    VisualBible    `json:"visual_bible,omitempty"`
	ObjectRegistry ObjectRegistry `json:"object_registry,omitempty"`
	Scenes         []ScenePlan    `json:"scenes"`
}
