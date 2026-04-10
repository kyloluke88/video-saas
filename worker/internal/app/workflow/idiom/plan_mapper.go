package idiom

import (
	"strings"

	idiommodel "worker/services/idiom/model"
)

func toServiceProjectPlan(plan idiommodel.ProjectPlanResult) idiommodel.RenderPlan {
	s := make([]idiommodel.RenderScene, 0, len(plan.Scenes))
	narrationLines := make([]string, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		sceneIndex := scene.SceneID
		if sceneIndex <= 0 {
			sceneIndex = len(s) + 1
		}
		action := make([]string, 0, len(scene.ActionUnit.Beats)+2)
		action = append(action, "起始: "+scene.ActionUnit.StartState)
		for _, beat := range scene.ActionUnit.Beats {
			line := beat.MicroAction
			line = "[" + beat.TRange + "] " + line
			action = append(action, line)
		}
		action = append(action, "结束: "+scene.ActionUnit.EndState)

		objectsRef := make([]string, 0, len(scene.Cast.Characters)+len(scene.Cast.Props)+1)
		objectsRef = append(objectsRef, scene.Cast.Characters...)
		objectsRef = append(objectsRef, scene.Cast.Props...)
		objectsRef = append(objectsRef, scene.Cast.Environment)

		dialogueParts := make([]string, 0, len(scene.AudioLayer.Dialogue))
		for _, d := range scene.AudioLayer.Dialogue {
			dialogueParts = append(dialogueParts, d.Content)
		}
		narration := strings.Join(dialogueParts, " ")
		narrationLines = append(narrationLines, narration)

		primaryCamera := idiommodel.CameraSpec{}
		if len(scene.Camera) > 0 {
			primaryCamera = scene.Camera[0]
		}

		composition := map[string]interface{}{}
		composition["shot_type"] = primaryCamera.ShotType
		composition["subject_blocking"] = []string{
			"angle: " + primaryCamera.Angle,
			"movement: " + primaryCamera.Movement,
			"focus: " + primaryCamera.Focus,
		}

		s = append(s, idiommodel.RenderScene{
			Index:       sceneIndex,
			DurationSec: scene.DurationSeconds,
			Goal:        scene.ActionUnit.UnitGoal,
			ObjectsRef:  objectsRef,
			Composition: composition,
			Action:      action,
			Prompt:      "",
			Narration:   narration,
		})
	}

	registryItems := make([]idiommodel.ObjectSpec, 0, len(plan.ObjectRegistry.Characters)+len(plan.ObjectRegistry.Props)+len(plan.ObjectRegistry.Environments))
	registryItems = append(registryItems, plan.ObjectRegistry.Characters...)
	registryItems = append(registryItems, plan.ObjectRegistry.Props...)
	registryItems = append(registryItems, plan.ObjectRegistry.Environments...)
	registry := make([]idiommodel.RenderObjectSpec, 0, len(registryItems))
	for _, item := range registryItems {
		itemType := "character"
		if containsObject(plan.ObjectRegistry.Props, item.ID) {
			itemType = "prop"
		}
		if containsObject(plan.ObjectRegistry.Environments, item.ID) {
			itemType = "environment"
		}
		registry = append(registry, idiommodel.RenderObjectSpec{
			ID:        item.ID,
			Type:      itemType,
			Label:     item.ID,
			Immutable: item.Immutable,
		})
	}
	return idiommodel.RenderPlan{
		ProjectID:         "",
		Platform:          "both",
		Category:          "idiom_story",
		NarrationLanguage: "zh-CN",
		TargetDurationSec: plan.Meta.TargetTotalSeconds,
		AspectRatio:       "16:9",
		Resolution:        "720p",
		Characters:        extractObjectIDs(plan.ObjectRegistry.Characters),
		Props:             extractObjectIDs(plan.ObjectRegistry.Props),
		SceneElements:     extractObjectIDs(plan.ObjectRegistry.Environments),
		NarrationFull:     strings.Join(narrationLines, " "),
		VisualBible: idiommodel.RenderVisualBible{
			StyleAnchor:       plan.VisualBible.StyleAnchor,
			CharacterAnchor:   plan.VisualBible.ColorPalette,
			EnvironmentAnchor: plan.VisualBible.EraSetting,
			NegativePrompt:    plan.VisualBible.NegativePrompt,
		},
		ObjectRegistry: registry,
		Scenes:         s,
		CreatedAt:      "",
	}
}

func extractObjectIDs(items []idiommodel.ObjectSpec) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func containsObject(items []idiommodel.ObjectSpec, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
