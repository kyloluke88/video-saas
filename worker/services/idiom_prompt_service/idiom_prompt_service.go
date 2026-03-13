package idiom_prompt_service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"worker/internal/dto"
)

type PromptVisualBible = dto.VisualBible
type PromptObjectRegistry = dto.ObjectRegistry
type PromptBeat = dto.Beat
type PromptActionUnit = dto.ActionUnit
type PromptDialogue = dto.Dialogue
type PromptSoundEffect = dto.SoundEffect
type PromptAudioLayer = dto.AudioLayer
type PromptCameraSpec = dto.CameraSpec
type PromptCast = dto.Cast
type PromptScene = dto.ScenePlan

type PromptPlan struct {
	VisualBible    PromptVisualBible
	ObjectRegistry PromptObjectRegistry
	Scenes         []PromptScene
}

type VisualBible struct {
	StyleAnchor    string `json:"style_anchor,omitempty"`
	ColorPalette   string `json:"color_palette,omitempty"`
	Lighting       string `json:"lighting,omitempty"`
	EraSetting     string `json:"era_setting,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

// BuildSceneObjectCatalog processes the object registry and returns three catalogs for characters, props, and environments.
func BuildSceneObjectCatalog(registry PromptObjectRegistry) (map[string]string, map[string]string, map[string]string) {
	characters := make(map[string]string)
	props := make(map[string]string)
	environments := make(map[string]string)

	for _, item := range registry.Characters {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		line := id
		if detail := formatImmutableForPrompt(item.Immutable); detail != "" {
			line = id + ": " + detail
		}
		characters[id] = line
	}
	for _, item := range registry.Props {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		line := id
		if detail := formatImmutableForPrompt(item.Immutable); detail != "" {
			line = id + ": " + detail
		}
		props[id] = line
	}
	for _, item := range registry.Environments {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		line := id
		if detail := formatImmutableForPrompt(item.Immutable); detail != "" {
			line = id + ": " + detail
		}
		environments[id] = line
	}

	return characters, props, environments
}

func BuildScenePrompt(scene dto.ScenePlan, characterCatalog, propsCatalog, environmentCatalog map[string]string, visualBible string) string {
	refs := make([]string, 0, len(scene.Cast.Characters)+len(scene.Cast.Props)+1)
	refs = append(refs, scene.Cast.Characters...)
	refs = append(refs, scene.Cast.Props...)

	if env := strings.TrimSpace(scene.Cast.Environment); env != "" {
		refs = append(refs, env)
	}
	characterLines := pickPromptLinesByRef(refs, characterCatalog)
	propsLines := pickPromptLinesByRef(refs, propsCatalog)
	environmentLines := pickPromptLinesByRef(refs, environmentCatalog)

	parts := make([]string, 0, 12)
	if len(environmentLines) > 0 {
		parts = append(parts, "[ENVIRONMENT]\n"+strings.Join(environmentLines, "\n"))
	}
	if len(characterLines) > 0 {
		parts = append(parts, "[CHARACTERS]\n"+strings.Join(characterLines, "\n"))
	}
	if len(propsLines) > 0 {
		parts = append(parts, "[PROPS]\n"+strings.Join(propsLines, "\n"))
	}
	if goal := strings.TrimSpace(scene.ActionUnit.UnitGoal); goal != "" {
		parts = append(parts, "[UNIT_GOAL]\n"+goal)
	}
	actionLines := make([]string, 0, len(scene.ActionUnit.Beats)+2)
	if start := strings.TrimSpace(scene.ActionUnit.StartState); start != "" {
		actionLines = append(actionLines, "起始: "+start)
	}
	for _, beat := range scene.ActionUnit.Beats {
		line := strings.TrimSpace(beat.MicroAction)
		if line == "" {
			continue
		}
		if tr := strings.TrimSpace(beat.TRange); tr != "" {
			line = "[" + tr + "] " + line
		}
		extra := make([]string, 0, 2)
		if v := strings.TrimSpace(beat.VisibleChange); v != "" {
			extra = append(extra, "visible_change: "+v)
		}
		if e := strings.TrimSpace(beat.EmotionalTone); e != "" {
			extra = append(extra, "emotional_tone: "+e)
		}
		if len(extra) > 0 {
			line += " (" + strings.Join(extra, "; ") + ")"
		}
		actionLines = append(actionLines, line)
	}
	if end := strings.TrimSpace(scene.ActionUnit.EndState); end != "" {
		actionLines = append(actionLines, "结束: "+end)
	}
	if len(actionLines) > 0 {
		parts = append(parts, "[TIMELINE]\n"+strings.Join(actionLines, "\n"))
	}
	audioLines := make([]string, 0, len(scene.AudioLayer.Dialogue)+len(scene.AudioLayer.SoundEffects)+1)
	for _, d := range scene.AudioLayer.Dialogue {
		content := strings.TrimSpace(d.Content)
		if content == "" {
			continue
		}
		line := content
		if speaker := strings.TrimSpace(d.Speaker); speaker != "" {
			line = speaker + ": " + line
		}
		if tr := strings.TrimSpace(d.TRange); tr != "" {
			line = "[" + tr + "] " + line
		}
		audioLines = append(audioLines, "dialogue: "+line)
	}
	for _, sfx := range scene.AudioLayer.SoundEffects {
		desc := strings.TrimSpace(sfx.Description)
		if desc == "" {
			continue
		}
		line := desc
		if tr := strings.TrimSpace(sfx.TRange); tr != "" {
			line = "[" + tr + "] " + line
		}
		audioLines = append(audioLines, "sound_effect: "+line)
	}
	if ambience := strings.TrimSpace(scene.AudioLayer.Ambience); ambience != "" {
		audioLines = append(audioLines, "ambience: "+ambience)
	}
	if len(audioLines) > 0 {
		parts = append(parts, "[AUDIO]\n"+strings.Join(audioLines, "\n"))
	}
	if comp := buildPromptComposition(scene.Camera); comp != "" {
		parts = append(parts, "[CAMERA]\n"+comp)
	}
	if transition := strings.TrimSpace(scene.TransitionToNext); transition != "" {
		parts = append(parts, "[TRANSITION]\n"+transition)
	}

	parts = append(parts, "[WORLD AND STYLE SETTING]\n"+visualBible)

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func BuildScenePromptJSON(plan PromptPlan, scene PromptScene, characterCatalog, propsCatalog, environmentCatalog map[string]string) string {
	refs := make([]string, 0, len(scene.Cast.Characters)+len(scene.Cast.Props)+1)
	refs = append(refs, scene.Cast.Characters...)
	refs = append(refs, scene.Cast.Props...)
	if env := strings.TrimSpace(scene.Cast.Environment); env != "" {
		refs = append(refs, env)
	}

	characterLines := pickPromptLinesByRef(refs, characterCatalog)
	propsLines := pickPromptLinesByRef(refs, propsCatalog)
	environmentLines := pickPromptLinesByRef(refs, environmentCatalog)

	timeline := make([]map[string]string, 0, len(scene.ActionUnit.Beats)+2)
	if start := strings.TrimSpace(scene.ActionUnit.StartState); start != "" {
		timeline = append(timeline, map[string]string{"type": "start_state", "value": start})
	}
	for _, beat := range scene.ActionUnit.Beats {
		item := map[string]string{}
		if tr := strings.TrimSpace(beat.TRange); tr != "" {
			item["t_range"] = tr
		}
		if action := strings.TrimSpace(beat.MicroAction); action != "" {
			item["micro_action"] = action
		}
		if vc := strings.TrimSpace(beat.VisibleChange); vc != "" {
			item["visible_change"] = vc
		}
		if et := strings.TrimSpace(beat.EmotionalTone); et != "" {
			item["emotional_tone"] = et
		}
		if len(item) > 0 {
			timeline = append(timeline, item)
		}
	}
	if end := strings.TrimSpace(scene.ActionUnit.EndState); end != "" {
		timeline = append(timeline, map[string]string{"type": "end_state", "value": end})
	}

	audioDialogue := make([]map[string]string, 0, len(scene.AudioLayer.Dialogue))
	for _, d := range scene.AudioLayer.Dialogue {
		item := map[string]string{}
		if sp := strings.TrimSpace(d.Speaker); sp != "" {
			item["speaker"] = sp
		}
		if tr := strings.TrimSpace(d.TRange); tr != "" {
			item["t_range"] = tr
		}
		if content := strings.TrimSpace(d.Content); content != "" {
			item["content"] = content
		}
		if len(item) > 0 {
			audioDialogue = append(audioDialogue, item)
		}
	}

	audioSFX := make([]map[string]string, 0, len(scene.AudioLayer.SoundEffects))
	for _, sfx := range scene.AudioLayer.SoundEffects {
		item := map[string]string{}
		if tr := strings.TrimSpace(sfx.TRange); tr != "" {
			item["t_range"] = tr
		}
		if desc := strings.TrimSpace(sfx.Description); desc != "" {
			item["description"] = desc
		}
		if len(item) > 0 {
			audioSFX = append(audioSFX, item)
		}
	}

	camera := make([]map[string]string, 0, len(scene.Camera))
	for _, seg := range scene.Camera {
		item := map[string]string{}
		if tr := strings.TrimSpace(seg.TRange); tr != "" {
			item["t_range"] = tr
		}
		if shot := strings.TrimSpace(seg.ShotType); shot != "" {
			item["shot_type"] = shot
		}
		if angle := strings.TrimSpace(seg.Angle); angle != "" {
			item["angle"] = angle
		}
		if movement := strings.TrimSpace(seg.Movement); movement != "" {
			item["movement"] = movement
		}
		if speed := strings.TrimSpace(seg.MovementSpeed); speed != "" {
			item["movement_speed"] = speed
		}
		if focus := strings.TrimSpace(seg.Focus); focus != "" {
			item["focus"] = focus
		}
		if comp := strings.TrimSpace(seg.Composition); comp != "" {
			item["composition"] = comp
		}
		if len(item) > 0 {
			camera = append(camera, item)
		}
	}

	style := make([]string, 0, 3)
	if v := strings.TrimSpace(plan.VisualBible.StyleAnchor); v != "" {
		style = append(style, v)
	}
	if v := strings.TrimSpace(plan.VisualBible.ColorPalette); v != "" {
		style = append(style, v)
	}
	if v := strings.TrimSpace(plan.VisualBible.Lighting); v != "" {
		style = append(style, v)
	}

	payload := map[string]interface{}{
		"environment":     environmentLines,
		"characters":      characterLines,
		"props":           propsLines,
		"unit_goal":       strings.TrimSpace(scene.ActionUnit.UnitGoal),
		"timeline":        timeline,
		"audio":           map[string]interface{}{"dialogue": audioDialogue, "sound_effects": audioSFX, "ambience": strings.TrimSpace(scene.AudioLayer.Ambience)},
		"camera":          camera,
		"transition":      strings.TrimSpace(scene.TransitionToNext),
		"world_setting":   strings.TrimSpace(plan.VisualBible.EraSetting),
		"style":           style,
		"negative_prompt": strings.TrimSpace(plan.VisualBible.NegativePrompt),
		"target_duration": scene.DurationSeconds,
		"scene_id":        scene.SceneID,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func pickPromptLinesByRef(refs []string, catalog map[string]string) []string {
	lines := make([]string, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		line, ok := catalog[ref]
		if !ok {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func buildPromptComposition(cameras []PromptCameraSpec) string {
	if len(cameras) == 0 {
		return ""
	}
	if len(cameras) == 1 {
		lines := buildPromptCompositionLines(cameras[0])
		return strings.Join(lines, "\n")
	}
	segments := make([]string, 0, len(cameras))
	for i, camera := range cameras {
		lines := make([]string, 0, 9)
		lines = append(lines, fmt.Sprintf("segment_%d:", i+1))
		lines = append(lines, buildPromptCompositionLines(camera)...)
		segments = append(segments, strings.Join(lines, "\n"))
	}
	return strings.Join(segments, "\n\n")
}

func buildPromptCompositionLines(camera PromptCameraSpec) []string {
	lines := make([]string, 0, 8)
	if tr := strings.TrimSpace(camera.TRange); tr != "" {
		lines = append(lines, "t_range: "+tr)
	}
	if shotType := strings.TrimSpace(camera.ShotType); shotType != "" {
		lines = append(lines, "shot_type: "+shotType)
	}
	if angle := strings.TrimSpace(camera.Angle); angle != "" {
		lines = append(lines, "angle: "+angle)
	}
	if movement := strings.TrimSpace(camera.Movement); movement != "" {
		lines = append(lines, "movement: "+movement)
	}
	if movementSpeed := strings.TrimSpace(camera.MovementSpeed); movementSpeed != "" {
		lines = append(lines, "movement_speed: "+movementSpeed)
	}
	if focus := strings.TrimSpace(camera.Focus); focus != "" {
		lines = append(lines, "focus: "+focus)
	}
	if composition := strings.TrimSpace(camera.Composition); composition != "" {
		lines = append(lines, "composition: "+composition)
	}
	return lines
}

func formatImmutableForPrompt(immutable map[string]interface{}) string {
	if len(immutable) == 0 {
		return ""
	}
	keys := make([]string, 0, len(immutable))
	for key := range immutable {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(immutable[key]))
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "; ")
}
