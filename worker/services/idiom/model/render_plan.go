package model

type RenderVisualBible struct {
	StyleAnchor       string `json:"style_anchor,omitempty"`
	CharacterAnchor   string `json:"character_anchor,omitempty"`
	EnvironmentAnchor string `json:"environment_anchor,omitempty"`
	NegativePrompt    string `json:"negative_prompt,omitempty"`
}

type RenderObjectSpec struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Immutable map[string]interface{} `json:"immutable,omitempty"`
	Mutable   map[string]interface{} `json:"mutable,omitempty"`
}

type RenderScene struct {
	Index       int                    `json:"index"`
	DurationSec int                    `json:"duration_sec"`
	Goal        string                 `json:"goal,omitempty"`
	ObjectsRef  []string               `json:"objects_ref,omitempty"`
	Composition map[string]interface{} `json:"composition,omitempty"`
	Action      []string               `json:"action,omitempty"`
	Prompt      string                 `json:"prompt"`
	Narration   string                 `json:"narration"`
}

type RenderPlan struct {
	ProjectID         string             `json:"project_id"`
	Platform          string             `json:"platform"`
	Category          string             `json:"category"`
	NarrationLanguage string             `json:"narration_language"`
	TargetDurationSec int                `json:"target_duration_sec"`
	AspectRatio       string             `json:"aspect_ratio"`
	Resolution        string             `json:"resolution"`
	Characters        []string           `json:"characters,omitempty"`
	Props             []string           `json:"props,omitempty"`
	SceneElements     []string           `json:"scene_elements,omitempty"`
	NarrationFull     string             `json:"narration_full"`
	VisualBible       RenderVisualBible  `json:"visual_bible,omitempty"`
	ObjectRegistry    []RenderObjectSpec `json:"object_registry,omitempty"`
	Scenes            []RenderScene      `json:"scenes"`
	CreatedAt         string             `json:"created_at"`
}
