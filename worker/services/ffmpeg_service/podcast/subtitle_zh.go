package podcast

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"worker/internal/dto"
)

func writeChineseASS(script dto.PodcastScript, projectDir, resolution string, style int) (string, error) {
	if len(script.Segments) == 0 {
		return "", nil
	}

	playW, playH := resolutionSize(resolution)
	layout := chineseSubtitleLayout(playW, playH, style)

	var b strings.Builder
	writeASSHeader(&b, layout)
	for _, seg := range script.Segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}

		tokens := segmentTokens(seg)
		if len(tokens) == 0 && strings.TrimSpace(seg.EN) == "" {
			continue
		}

		start := formatASSTimestampMS(seg.StartMS)
		end := formatASSTimestampMS(seg.EndMS)
		hasRuby := hasAnyRuby(tokens)
		hasEnglish := strings.TrimSpace(seg.EN) != ""

		tokenLines := splitTokenLines(buildTokenCells(tokens, layout), layout.MaxTextWidth, 2)
		englishLines := splitEnglishLines(strings.TrimSpace(seg.EN), layout, 2)

		b.WriteString(dialogueLine("Box", start, end, boxText(layout)))

		topRows := computeTopSectionRows(layout, len(tokenLines), hasRuby)
		for lineIndex, line := range tokenLines {
			if lineIndex >= len(topRows) {
				break
			}
			positions := computeCellCenters(line, layout)
			row := topRows[lineIndex]
			for i, cell := range line {
				x := positions[i]
				if strings.TrimSpace(cell.Hanzi) != "" {
					b.WriteString(dialogueLine("Hanzi", start, end, posText(x, row.HanziY, cell.Hanzi)))
					if cell.EndMS > cell.StartMS {
						b.WriteString(dialogueLine("HanziActive", formatASSTimestampMS(cell.StartMS), formatASSTimestampMS(cell.EndMS), posText(x, row.HanziY, cell.Hanzi)))
					}
				}
				if strings.TrimSpace(cell.Ruby) != "" {
					b.WriteString(dialogueLine("Ruby", start, end, posText(x, row.RubyY, cell.Ruby)))
				}
			}
		}

		if hasEnglish {
			englishRows := computeBottomSectionRows(layout, len(englishLines))
			for i, line := range englishLines {
				if i >= len(englishRows) {
					break
				}
				b.WriteString(dialogueLine("English", start, end, posText(playW/2, englishRows[i], line)))
			}
		}
	}

	if b.Len() == 0 {
		return "", nil
	}

	out := filepath.Join(projectDir, "podcast_subtitles.ass")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

type subtitleLayout struct {
	PlayW               int
	PlayH               int
	BoxLeft             int
	BoxTop              int
	BoxWidth            int
	BoxHeight           int
	MaxTextWidth        int
	RubySize            int
	HanziSize           int
	EnglishSize         int
	BaseGap             int
	RowGap              int
	TokenLineGap        int
	EnglishLineGap      int
	TopSectionTop       int
	TopSectionHeight    int
	BottomSectionTop    int
	BottomSectionHeight int
	BoxColor            string
	RubyColor           string
	HanziColor          string
	EnglishColor        string
	OutlineColor        string
	RubyFontName        string
	HanziFontName       string
	EnglishFontName     string
	RubyBold            int
	HanziBold           int
	EnglishBold         int
}

func newSubtitleLayout(playW, playH int, preset subtitlePreset) subtitleLayout {
	scale := float64(playH) / 1080.0

	boxLeft := int(float64(playW) * preset.BoxLeftRatio)
	boxTop := int(float64(playH) * preset.BoxTopRatio)
	boxWidth := int(float64(playW) * preset.BoxWidthRatio)
	if boxWidth <= 0 {
		boxWidth = playW
	}
	boxHeight := int(float64(playH) * preset.BoxHeightRatio)
	if boxHeight <= 0 {
		boxHeight = playH - boxTop
	}
	topSectionHeight := int(float64(boxHeight) * preset.TopSectionRatio)
	bottomSectionTop := boxTop + topSectionHeight
	bottomSectionHeight := int(float64(boxHeight) * preset.BottomSectionRatio)

	return subtitleLayout{
		PlayW:               playW,
		PlayH:               playH,
		BoxLeft:             boxLeft,
		BoxTop:              boxTop,
		BoxWidth:            boxWidth,
		BoxHeight:           boxHeight,
		MaxTextWidth:        int(float64(boxWidth) * preset.TextWidthRatio),
		RubySize:            maxInt(16, int(float64(preset.RubySize)*scale)),
		HanziSize:           maxInt(24, int(float64(preset.HanziSize)*scale)),
		EnglishSize:         maxInt(18, int(float64(preset.EnglishSize)*scale)),
		BaseGap:             maxInt(2, int(float64(preset.BaseGap)*scale)),
		RowGap:              maxInt(4, int(float64(preset.RowGap)*scale)),
		TokenLineGap:        maxInt(6, int(float64(preset.TokenLineGap)*scale)),
		EnglishLineGap:      maxInt(4, int(float64(preset.EnglishLineGap)*scale)),
		TopSectionTop:       boxTop + int(float64(boxHeight)*preset.TopSectionTopInset),
		TopSectionHeight:    topSectionHeight,
		BottomSectionTop:    bottomSectionTop + int(float64(boxHeight)*preset.BottomSectionTopInset),
		BottomSectionHeight: bottomSectionHeight,
		BoxColor:            preset.BoxColor,
		RubyColor:           preset.RubyColor,
		HanziColor:          preset.HanziColor,
		EnglishColor:        preset.EnglishColor,
		OutlineColor:        preset.OutlineColor,
		RubyFontName:        preset.RubyFontName,
		HanziFontName:       preset.HanziFontName,
		EnglishFontName:     preset.EnglishFontName,
		RubyBold:            preset.RubyBold,
		HanziBold:           preset.HanziBold,
		EnglishBold:         preset.EnglishBold,
	}
}

func chineseSubtitleLayout(playW, playH, style int) subtitleLayout {
	return newSubtitleLayout(playW, playH, chineseSubtitlePresetFor(style))
}

type subtitlePreset struct {
	RubyFontName          string
	HanziFontName         string
	EnglishFontName       string
	BoxLeftRatio          float64
	BoxTopRatio           float64
	BoxWidthRatio         float64
	BoxHeightRatio        float64
	TextWidthRatio        float64
	TopSectionRatio       float64
	BottomSectionRatio    float64
	TopSectionTopInset    float64
	BottomSectionTopInset float64
	RubySize              int
	HanziSize             int
	EnglishSize           int
	BaseGap               int
	RowGap                int
	TokenLineGap          int
	EnglishLineGap        int
	BoxColor              string
	RubyColor             string
	HanziColor            string
	EnglishColor          string
	OutlineColor          string
	RubyBold              int
	HanziBold             int
	EnglishBold           int
}

func chineseSubtitlePresetFor(style int) subtitlePreset {
	switch style {
	case 1:
		return subtitlePreset{
			RubyFontName:          "Tenor Sans",
			HanziFontName:         "ChillKai Regular",
			EnglishFontName:       "Jacques Francois",
			BoxLeftRatio:          0,
			BoxTopRatio:           0.56,
			BoxWidthRatio:         1,
			BoxHeightRatio:        0.40,
			TextWidthRatio:        0.90,
			TopSectionRatio:       0.60,
			BottomSectionRatio:    0.24,
			TopSectionTopInset:    0.06,
			BottomSectionTopInset: 0.02,
			RubySize:              42,
			HanziSize:             70,
			EnglishSize:           42,
			BaseGap:               4,
			RowGap:                10,
			TokenLineGap:          18,
			EnglishLineGap:        8,
			BoxColor:              "&HFF000000",
			RubyColor:             "&H00EDF4F7",
			HanziColor:            "&H00F3F8FA",
			EnglishColor:          "&H0019D8FF",
			OutlineColor:          "&H003B5B55",
			RubyBold:              0,
			HanziBold:             1,
			EnglishBold:           1,
		}
	case 2:
		return subtitlePreset{
			RubyFontName:          "Tenor Sans",
			HanziFontName:         "HYWenRunSongYun J",
			EnglishFontName:       "Radley",
			BoxLeftRatio:          0,
			BoxTopRatio:           0.415,
			BoxWidthRatio:         1,
			BoxHeightRatio:        0.535,
			TextWidthRatio:        0.92,
			TopSectionRatio:       0.748,
			BottomSectionRatio:    0.252,
			TopSectionTopInset:    0,
			BottomSectionTopInset: 0,
			RubySize:              42,
			HanziSize:             70,
			EnglishSize:           42,
			BaseGap:               4,
			RowGap:                12,
			TokenLineGap:          18,
			EnglishLineGap:        10,
			BoxColor:              "&HFF000000",
			RubyColor:             "&H00686868",
			HanziColor:            "&H00434343",
			EnglishColor:          "&H008878D8",
			OutlineColor:          "&H00DDD6CF",
			RubyBold:              0,
			HanziBold:             1,
			EnglishBold:           1,
		}
	default:
		return subtitlePreset{
			RubyFontName:          "Noto Sans CJK SC",
			HanziFontName:         "Noto Sans CJK SC",
			EnglishFontName:       "Noto Sans",
			BoxLeftRatio:          0.03,
			BoxTopRatio:           0.70,
			BoxWidthRatio:         0.94,
			BoxHeightRatio:        0.20,
			TextWidthRatio:        0.90,
			TopSectionRatio:       0.54,
			BottomSectionRatio:    0.26,
			TopSectionTopInset:    0.07,
			BottomSectionTopInset: 0.02,
			RubySize:              42,
			HanziSize:             70,
			EnglishSize:           42,
			BaseGap:               4,
			RowGap:                10,
			TokenLineGap:          14,
			EnglishLineGap:        8,
			BoxColor:              "&H3A000000",
			RubyColor:             "&H00EAEAEA",
			HanziColor:            "&H00FFFFFF",
			EnglishColor:          "&H00A8F9FF",
			OutlineColor:          "&H00101010",
			RubyBold:              0,
			HanziBold:             1,
			EnglishBold:           1,
		}
	}
}

func segmentTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	return seg.Tokens
}

func formatASSTimestampMS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	cs := ms / 10
	h := cs / 360000
	m := (cs % 360000) / 6000
	s := (cs % 6000) / 100
	c := cs % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, c)
}

func dialogueLine(style, start, end, text string) string {
	return fmt.Sprintf("Dialogue: 0,%s,%s,%s,,0,0,0,,%s\n", start, end, style, text)
}

func boxText(layout subtitleLayout) string {
	return fmt.Sprintf("{\\an7\\pos(%d,%d)\\p1}m 0 0 l %d 0 l %d %d l 0 %d{\\p0}",
		layout.BoxLeft, layout.BoxTop, layout.BoxWidth, layout.BoxWidth, layout.BoxHeight, layout.BoxHeight)
}

func posText(x, y int, text string) string {
	return fmt.Sprintf("{\\an5\\pos(%d,%d)}%s", x, y, escapeASSText(text))
}

func escapeASSText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	s = strings.ReplaceAll(s, "\n", `\N`)
	return s
}

func writeASSHeader(b *strings.Builder, layout subtitleLayout) {
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	b.WriteString("Collisions: Normal\n")
	b.WriteString("PlayDepth: 0\n")
	b.WriteString("WrapStyle: 2\n")
	b.WriteString("ScaledBorderAndShadow: yes\n")
	b.WriteString("YCbCr Matrix: TV.601\n")
	b.WriteString("PlayResX: " + strconv.Itoa(layout.PlayW) + "\n")
	b.WriteString("PlayResY: " + strconv.Itoa(layout.PlayH) + "\n\n")

	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\n")
	b.WriteString("Style: Box," + layout.HanziFontName + ",20," + layout.BoxColor + "," + layout.BoxColor + "," + layout.BoxColor + "," + layout.BoxColor + ",0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n")
	b.WriteString("Style: Ruby," + layout.RubyFontName + "," + strconv.Itoa(layout.RubySize) + "," + layout.RubyColor + "," + layout.RubyColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.RubyBold) + ",0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: Hanzi," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HanziColor + "," + layout.HanziColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.HanziBold) + ",0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: HanziActive," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + ",&H0078ECFF,&H0078ECFF," + layout.OutlineColor + ",&H64000000,0,0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: English," + layout.EnglishFontName + "," + strconv.Itoa(layout.EnglishSize) + "," + layout.EnglishColor + "," + layout.EnglishColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.EnglishBold) + ",0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n\n")

	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n")
}

type tokenCell struct {
	Hanzi   string
	Ruby    string
	Width   float64
	Gap     float64
	StartMS int
	EndMS   int
}

type topSectionRow struct {
	RubyY  int
	HanziY int
}

func buildTokenCells(tokens []dto.PodcastToken, layout subtitleLayout) []tokenCell {
	out := make([]tokenCell, 0, len(tokens))
	for _, tk := range tokens {
		hanzi := strings.TrimSpace(tk.Char)
		ruby := strings.TrimSpace(tk.Pinyin)
		if hanzi == "" && ruby == "" {
			continue
		}
		hanziW := estimateTextWidth(hanzi, float64(layout.HanziSize), true)
		rubyW := estimateTextWidth(ruby, float64(layout.RubySize), false)
		w := maxFloat(hanziW, rubyW)
		if isPunctuationText(hanzi) {
			w = maxFloat(w*0.60, float64(layout.HanziSize)*0.48)
		}
		gap := float64(layout.BaseGap)
		if isPunctuationText(hanzi) {
			gap = float64(layout.BaseGap) * 0.30
		}
		out = append(out, tokenCell{Hanzi: hanzi, Ruby: ruby, Width: w, Gap: gap, StartMS: tk.StartMS, EndMS: tk.EndMS})
	}
	return out
}

func splitTokenLines(cells []tokenCell, maxWidth int, maxLines int) [][]tokenCell {
	if len(cells) == 0 {
		return nil
	}
	lines := make([][]tokenCell, 0, maxLines)
	current := make([]tokenCell, 0, len(cells))
	currentWidth := 0.0
	limit := float64(maxWidth)

	for _, cell := range cells {
		nextWidth := currentWidth
		if len(current) > 0 {
			nextWidth += current[len(current)-1].Gap
		}
		nextWidth += cell.Width
		if len(current) > 0 && nextWidth > limit && len(lines) < maxLines-1 {
			lines = append(lines, current)
			current = []tokenCell{cell}
			currentWidth = cell.Width
			continue
		}
		if len(current) > 0 {
			currentWidth += current[len(current)-1].Gap
		}
		current = append(current, cell)
		currentWidth += cell.Width
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		tail := make([]tokenCell, 0)
		for _, extra := range lines[maxLines-1:] {
			tail = append(tail, extra...)
		}
		lines = append(lines[:maxLines-1], tail)
	}
	return lines
}

func computeCellCenters(cells []tokenCell, layout subtitleLayout) []int {
	if len(cells) == 0 {
		return nil
	}
	total := 0.0
	for i, c := range cells {
		total += c.Width
		if i != len(cells)-1 {
			total += c.Gap
		}
	}
	maxW := float64(layout.MaxTextWidth)
	scale := 1.0
	if total > maxW && total > 0 {
		scale = maxW / total
	}
	start := float64(layout.PlayW)/2.0 - (total*scale)/2.0
	out := make([]int, 0, len(cells))
	cursor := start
	for i, c := range cells {
		w := c.Width * scale
		out = append(out, int(cursor+w/2.0))
		cursor += w
		if i != len(cells)-1 {
			cursor += c.Gap * scale
		}
	}
	return out
}

func computeTopSectionRows(layout subtitleLayout, lineCount int, hasRuby bool) []topSectionRow {
	if lineCount <= 0 {
		return nil
	}
	blockHeight := int(float64(layout.HanziSize) * 1.05)
	if hasRuby {
		blockHeight += int(float64(layout.RubySize)*1.12) + layout.RowGap
	}
	total := lineCount * blockHeight
	if lineCount > 1 {
		total += (lineCount - 1) * layout.TokenLineGap
	}
	start := layout.TopSectionTop + (layout.TopSectionHeight-total)/2
	rows := make([]topSectionRow, 0, lineCount)
	cursor := start
	for i := 0; i < lineCount; i++ {
		row := topSectionRow{RubyY: -1, HanziY: -1}
		if hasRuby {
			rubyH := int(float64(layout.RubySize) * 1.12)
			row.RubyY = cursor + rubyH/2
			cursor += rubyH + layout.RowGap
		}
		hanziH := int(float64(layout.HanziSize) * 1.05)
		row.HanziY = cursor + hanziH/2
		cursor += hanziH
		rows = append(rows, row)
		if i != lineCount-1 {
			cursor += layout.TokenLineGap
		}
	}
	return rows
}

func splitEnglishLines(text string, layout subtitleLayout, maxLines int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}
	lines := make([]string, 0, maxLines)
	current := ""
	limit := float64(layout.MaxTextWidth) * 0.96
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if current != "" && estimateTextWidth(candidate, float64(layout.EnglishSize), false) > limit && len(lines) < maxLines-1 {
			lines = append(lines, current)
			current = word
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		tail := strings.Join(lines[maxLines-1:], " ")
		lines = append(lines[:maxLines-1], tail)
	}
	return lines
}

func computeBottomSectionRows(layout subtitleLayout, lineCount int) []int {
	if lineCount <= 0 {
		return nil
	}
	rowHeight := int(float64(layout.EnglishSize) * 1.14)
	total := lineCount * rowHeight
	if lineCount > 1 {
		total += (lineCount - 1) * layout.EnglishLineGap
	}
	start := layout.BottomSectionTop + (layout.BottomSectionHeight-total)/2
	out := make([]int, 0, lineCount)
	cursor := start
	for i := 0; i < lineCount; i++ {
		out = append(out, cursor+rowHeight/2)
		cursor += rowHeight
		if i != lineCount-1 {
			cursor += layout.EnglishLineGap
		}
	}
	return out
}

func estimateTextWidth(text string, fontSize float64, cjk bool) float64 {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	runes := []rune(text)
	width := 0.0
	for _, r := range runes {
		switch {
		case unicode.IsSpace(r):
			width += fontSize * 0.28
		case isPunctuationRune(r):
			width += fontSize * 0.36
		case cjk:
			width += fontSize * 0.94
		default:
			width += fontSize * 0.58
		}
	}
	return width
}

func hasAnyRuby(tokens []dto.PodcastToken) bool {
	for _, t := range tokens {
		if strings.TrimSpace(t.Pinyin) != "" {
			return true
		}
	}
	return false
}

func isPunctuationText(s string) bool {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) != 1 {
		return false
	}
	return isPunctuationRune(rs[0])
}

func isPunctuationRune(r rune) bool {
	return unicode.IsPunct(r) || strings.ContainsRune("，。！？；：“”‘’（）《》、…", r)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func resolutionSize(resolution string) (int, int) {
	switch strings.TrimSpace(strings.ToLower(resolution)) {
	case "480p":
		return 854, 480
	case "720p":
		return 1280, 720
	case "1440p":
		return 2560, 1440
	case "2000p":
		return 3556, 2000
	default:
		return 1920, 1080
	}
}
