package model

type PracticalImagePlan struct {
	Version     string                   `json:"version,omitempty"`
	Provider    string                   `json:"provider,omitempty"`
	Model       string                   `json:"model,omitempty"`
	Resolution  string                   `json:"resolution,omitempty"`
	AspectRatio string                   `json:"aspect_ratio,omitempty"`
	Blocks      []PracticalImagePlanItem `json:"blocks,omitempty"`
	Chapters    []PracticalImagePlanItem `json:"chapters,omitempty"`
}

type PracticalImagePlanItem struct {
	AssetKey     string                       `json:"asset_key,omitempty"`
	AssetType    string                       `json:"asset_type,omitempty"`
	BlockID      string                       `json:"block_id,omitempty"`
	ChapterID    string                       `json:"chapter_id,omitempty"`
	SourcePrompt string                       `json:"source_prompt,omitempty"`
	Prompt       string                       `json:"prompt,omitempty"`
	Filename     string                       `json:"filename,omitempty"`
	References   []PracticalImageReferenceRef `json:"references,omitempty"`
}

type PracticalImageReferenceRef struct {
	Slot      int    `json:"slot,omitempty"`
	Role      string `json:"role,omitempty"`
	SpeakerID string `json:"speaker_id,omitempty"`
	View      string `json:"view,omitempty"`
	Filename  string `json:"filename,omitempty"`
}

type PracticalImageManifest struct {
	Version     string                `json:"version,omitempty"`
	Provider    string                `json:"provider,omitempty"`
	Model       string                `json:"model,omitempty"`
	Resolution  string                `json:"resolution,omitempty"`
	AspectRatio string                `json:"aspect_ratio,omitempty"`
	Blocks      []PracticalImageAsset `json:"blocks,omitempty"`
	Chapters    []PracticalImageAsset `json:"chapters,omitempty"`
}

type PracticalImageAsset struct {
	AssetKey     string                       `json:"asset_key,omitempty"`
	AssetType    string                       `json:"asset_type,omitempty"`
	BlockID      string                       `json:"block_id,omitempty"`
	ChapterID    string                       `json:"chapter_id,omitempty"`
	Filename     string                       `json:"filename,omitempty"`
	SourcePrompt string                       `json:"source_prompt,omitempty"`
	Prompt       string                       `json:"prompt,omitempty"`
	References   []PracticalImageReferenceRef `json:"references,omitempty"`
}
