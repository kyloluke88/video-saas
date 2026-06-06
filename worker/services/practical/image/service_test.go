package practical_image_service

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
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

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
}

func decodeImageConfig(t *testing.T, path string) (image.Config, string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode config %s: %v", path, err)
	}
	return cfg, format
}

func TestGenerateFromLocalAssetsNormalizesExpectedImages(t *testing.T) {
	requireFFmpeg(t)

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

	result, err := generateFromLocalAssets(context.Background(), projectDir, script, script.Blocks, "1080p", "16:9")
	if err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}
	blockPath := projectImageAbsolutePath(projectDir, "images/blocks/block_01.png")
	chapterPath := projectImageAbsolutePath(projectDir, "images/chapters/ch_01.png")
	if !fileExists(blockPath) {
		t.Fatalf("expected copied block image")
	}
	if !fileExists(chapterPath) {
		t.Fatalf("expected copied chapter image")
	}
	blockCfg, blockFormat := decodeImageConfig(t, blockPath)
	chapterCfg, chapterFormat := decodeImageConfig(t, chapterPath)
	if blockCfg.Width != 1920 || blockCfg.Height != 1080 || blockFormat != "png" {
		t.Fatalf("unexpected block output: %#v format=%s", blockCfg, blockFormat)
	}
	if chapterCfg.Width != 1920 || chapterCfg.Height != 1080 || chapterFormat != "png" {
		t.Fatalf("unexpected chapter output: %#v format=%s", chapterCfg, chapterFormat)
	}
	if got := result.Manifest.Provider; got != "local-assets" {
		t.Fatalf("unexpected provider: %s", got)
	}
	if len(result.Manifest.Blocks) != 1 || len(result.Manifest.Chapters) != 1 {
		t.Fatalf("unexpected manifest counts: %#v", result.Manifest)
	}
}

func TestGenerateFromLocalAssetsNormalizesDeclaredExtension(t *testing.T) {
	requireFFmpeg(t)

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

	writeJPEGNamedAsPNG := func(path string, c color.RGBA) {
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
			t.Fatalf("encode jpeg %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
	}

	writeJPEGNamedAsPNG(filepath.Join(bgDir, "blocks", "block_01.png"), color.RGBA{R: 255, A: 255})
	writeJPEGNamedAsPNG(filepath.Join(bgDir, "chapters", "ch_01.png"), color.RGBA{B: 255, A: 255})

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

	if _, err := generateFromLocalAssets(context.Background(), projectDir, script, script.Blocks, "720p", "16:9"); err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}

	targetBlock := projectImageAbsolutePath(projectDir, "images/blocks/block_01.png")
	targetChapter := projectImageAbsolutePath(projectDir, "images/chapters/ch_01.png")
	blockCfg, blockFormat := decodeImageConfig(t, targetBlock)
	chapterCfg, chapterFormat := decodeImageConfig(t, targetChapter)
	if blockCfg.Width != 1280 || blockCfg.Height != 720 || blockFormat != "png" {
		t.Fatalf("unexpected block output: %#v format=%s", blockCfg, blockFormat)
	}
	if chapterCfg.Width != 1280 || chapterCfg.Height != 720 || chapterFormat != "png" {
		t.Fatalf("unexpected chapter output: %#v format=%s", chapterCfg, chapterFormat)
	}
}

func TestGenerateFromLocalAssetsUsesJPEGNamedAssets(t *testing.T) {
	requireFFmpeg(t)

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

	result, err := generateFromLocalAssets(context.Background(), projectDir, script, script.Blocks, "1080p", "16:9")
	if err != nil {
		t.Fatalf("generateFromLocalAssets returned error: %v", err)
	}
	if got := result.Manifest.Blocks[0].Filename; got != "images/blocks/block_01.jpeg" {
		t.Fatalf("unexpected block filename: %s", got)
	}
	if got := result.Manifest.Chapters[0].Filename; got != "images/chapters/ch_01.jpeg" {
		t.Fatalf("unexpected chapter filename: %s", got)
	}
	blockPath := projectImageAbsolutePath(projectDir, "images/blocks/block_01.jpeg")
	chapterPath := projectImageAbsolutePath(projectDir, "images/chapters/ch_01.jpeg")
	if !fileExists(blockPath) {
		t.Fatalf("expected copied jpeg block image")
	}
	if !fileExists(chapterPath) {
		t.Fatalf("expected copied jpeg chapter image")
	}
	blockCfg, blockFormat := decodeImageConfig(t, blockPath)
	chapterCfg, chapterFormat := decodeImageConfig(t, chapterPath)
	if blockCfg.Width != 1920 || blockCfg.Height != 1080 || blockFormat != "jpeg" {
		t.Fatalf("unexpected block output: %#v format=%s", blockCfg, blockFormat)
	}
	if chapterCfg.Width != 1920 || chapterCfg.Height != 1080 || chapterFormat != "jpeg" {
		t.Fatalf("unexpected chapter output: %#v format=%s", chapterCfg, chapterFormat)
	}
}
