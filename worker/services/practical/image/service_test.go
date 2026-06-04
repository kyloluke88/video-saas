package practical_image_service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	dto "worker/services/practical/model"
)

func TestBuildStaticImagePlanUsesBundledPNGAssets(t *testing.T) {
	plan := buildStaticImagePlan([]dto.PracticalBlock{
		{
			BlockID:     "block_01",
			BlockPrompt: "block prompt",
			Chapters: []dto.PracticalChapter{
				{
					ChapterID:   "ch_01",
					ScenePrompt: "scene prompt",
				},
			},
		},
	}, "1080p", "16:9")

	if len(plan.Blocks) != 1 || plan.Blocks[0].Filename != "images/blocks/block_01.png" {
		t.Fatalf("unexpected block plan: %#v", plan.Blocks)
	}
	if len(plan.Chapters) != 1 || plan.Chapters[0].Filename != "images/chapters/ch_01.png" {
		t.Fatalf("unexpected chapter plan: %#v", plan.Chapters)
	}
}

func TestGenerateFromLocalAssetsCopiesExpectedImages(t *testing.T) {
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	bgDir := filepath.Join(tmpDir, "assets", "practical", "bg-images")
	for _, subdir := range []string{"blocks", "chapters"} {
		if err := os.MkdirAll(filepath.Join(bgDir, subdir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", subdir, err)
		}
	}

	writePNG := func(path string) {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		img.Set(0, 0, color.RGBA{B: 255, A: 255})
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := png.Encode(file, img); err != nil {
			_ = file.Close()
			t.Fatalf("encode %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
	}
	writePNG(filepath.Join(bgDir, "blocks", "block_01.png"))
	writePNG(filepath.Join(bgDir, "chapters", "ch_01.png"))

	projectDir := projectDirFor("local-demo")
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID:     "block_01",
				Topic:       "topic",
				BlockPrompt: "block prompt",
				Speakers: []dto.PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "clerk"},
				},
				Chapters: []dto.PracticalChapter{
					{
						ChapterID:   "ch_01",
						Scene:       "scene",
						ScenePrompt: "scene prompt",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", SpeakerRole: "customer", Text: "こんにちは。"},
						},
					},
				},
			},
		},
	}

	result, err := generateFromLocalAssets(projectDir, script, script.Blocks, "1080p", "16:9")
	if err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}
	if !fileExists(projectImageAbsolutePath(projectDir, "images/blocks/block_01.png")) {
		t.Fatalf("expected copied block image")
	}
	if !fileExists(projectImageAbsolutePath(projectDir, "images/chapters/ch_01.png")) {
		t.Fatalf("expected copied chapter image")
	}
	if got := result.Manifest.Provider; got != "local-assets" {
		t.Fatalf("unexpected provider: %s", got)
	}
	if len(result.Manifest.Blocks) != 1 || len(result.Manifest.Chapters) != 1 {
		t.Fatalf("unexpected manifest counts: %#v", result.Manifest)
	}
}

func TestGenerateFromLocalAssetsPreservesOriginalBytes(t *testing.T) {
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	bgDir := filepath.Join(tmpDir, "assets", "practical", "bg-images")
	for _, subdir := range []string{"blocks", "chapters"} {
		if err := os.MkdirAll(filepath.Join(bgDir, subdir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", subdir, err)
		}
	}

	writeJPEGNamedAsPNG := func(path string, c color.RGBA) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				img.Set(x, y, c)
			}
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			t.Fatalf("encode jpeg %s: %v", path, err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return buf.Bytes()
	}

	sourceBlock := writeJPEGNamedAsPNG(filepath.Join(bgDir, "blocks", "block_01.png"), color.RGBA{R: 255, A: 255})
	sourceChapter := writeJPEGNamedAsPNG(filepath.Join(bgDir, "chapters", "ch_01.png"), color.RGBA{B: 255, A: 255})

	projectDir := projectDirFor("local-preserve")
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID:     "block_01",
				Topic:       "topic",
				BlockPrompt: "block prompt",
				Speakers: []dto.PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "clerk"},
				},
				Chapters: []dto.PracticalChapter{
					{
						ChapterID:   "ch_01",
						Scene:       "scene",
						ScenePrompt: "scene prompt",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", SpeakerRole: "customer", Text: "こんにちは。"},
						},
					},
				},
			},
		},
	}

	if _, err := generateFromLocalAssets(projectDir, script, script.Blocks, "720p", "16:9"); err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}

	targetBlock := projectImageAbsolutePath(projectDir, "images/blocks/block_01.png")
	targetChapter := projectImageAbsolutePath(projectDir, "images/chapters/ch_01.png")
	gotBlock, err := os.ReadFile(targetBlock)
	if err != nil {
		t.Fatalf("read target block: %v", err)
	}
	gotChapter, err := os.ReadFile(targetChapter)
	if err != nil {
		t.Fatalf("read target chapter: %v", err)
	}
	if !bytes.Equal(gotBlock, sourceBlock) {
		t.Fatalf("block asset bytes changed during local copy")
	}
	if !bytes.Equal(gotChapter, sourceChapter) {
		t.Fatalf("chapter asset bytes changed during local copy")
	}
}

func TestGenerateFromLocalAssetsUsesJPEGNamedAssets(t *testing.T) {
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	bgDir := filepath.Join(tmpDir, "assets", "practical", "bg-images")
	for _, subdir := range []string{"blocks", "chapters"} {
		if err := os.MkdirAll(filepath.Join(bgDir, subdir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", subdir, err)
		}
	}

	writeJPEG := func(path string, c color.RGBA) {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				img.Set(x, y, c)
			}
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 85}); err != nil {
			_ = file.Close()
			t.Fatalf("encode %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
	}
	writeJPEG(filepath.Join(bgDir, "blocks", "block_01.jpeg"), color.RGBA{R: 255, A: 255})
	writeJPEG(filepath.Join(bgDir, "chapters", "ch_01.jpeg"), color.RGBA{B: 255, A: 255})

	projectDir := projectDirFor("local-jpeg")
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID:     "block_01",
				Topic:       "topic",
				BlockPrompt: "block prompt",
				Speakers: []dto.PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "clerk"},
				},
				Chapters: []dto.PracticalChapter{
					{
						ChapterID:   "ch_01",
						Scene:       "scene",
						ScenePrompt: "scene prompt",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", SpeakerRole: "customer", Text: "こんにちは。"},
						},
					},
				},
			},
		},
	}

	result, err := generateFromLocalAssets(projectDir, script, script.Blocks, "1080p", "16:9")
	if err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}
	if got := result.Manifest.Blocks[0].Filename; got != "images/blocks/block_01.jpeg" {
		t.Fatalf("unexpected block filename: %s", got)
	}
	if got := result.Manifest.Chapters[0].Filename; got != "images/chapters/ch_01.jpeg" {
		t.Fatalf("unexpected chapter filename: %s", got)
	}
	if !fileExists(projectImageAbsolutePath(projectDir, "images/blocks/block_01.jpeg")) {
		t.Fatalf("expected copied jpeg block image")
	}
	if !fileExists(projectImageAbsolutePath(projectDir, "images/chapters/ch_01.jpeg")) {
		t.Fatalf("expected copied jpeg chapter image")
	}
}
