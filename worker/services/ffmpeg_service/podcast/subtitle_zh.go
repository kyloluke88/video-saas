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

const (
	designType1ReferenceHeight    = 775.0
	designType1TopBandTopRatio    = 55.0 / designType1ReferenceHeight
	designType1TopBandBottomRatio = 185.0 / designType1ReferenceHeight
	designType1ENBandBottomRatio  = 270.0 / designType1ReferenceHeight
	designType1BoxHeightRatio     = designType1ENBandBottomRatio - designType1TopBandTopRatio
	designType1TopSectionRatio    = (designType1TopBandBottomRatio - designType1TopBandTopRatio) / designType1BoxHeightRatio
	designType1BottomSectionRatio = (designType1ENBandBottomRatio - designType1TopBandBottomRatio) / designType1BoxHeightRatio
)

func writeChineseASS(script dto.PodcastScript, projectDir, resolution string, style int) (string, error) {
	if len(script.Segments) == 0 {
		return "", nil
	}

	playW, playH := resolutionSize(resolution)
	layout := chineseSubtitleLayout(playW, playH, style)
	highlightEnabled := podcastHighlightEnabled()

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
		hasEnglish := strings.TrimSpace(seg.EN) != ""

		tokenPages := paginateTokenCells(buildTokenCells(tokens, layout), layout)
		pageStartTimes := chinesePageStartTimes(tokenPages)
		if !highlightEnabled {
			pageStartTimes = nil
		}

		b.WriteString(dialogueLine("Box", start, end, boxText(layout)))

		pageWindows := buildSubtitlePageWindows(seg.StartMS, seg.EndMS, pageStartTimes, chinesePageWeights(tokenPages))
		for pageIndex, page := range tokenPages {
			if pageIndex >= len(pageWindows) || len(page) == 0 {
				break
			}
			rows := computeTopSectionRows(layout, 1, chinesePageHasRuby(page))
			if len(rows) == 0 {
				continue
			}
			row := rows[0]
			window := pageWindows[pageIndex]
			pageStart := formatASSTimestampMS(window.StartMS)
			pageEnd := formatASSTimestampMS(window.EndMS)
			positions := computeCellCenters(page, layout)
			for i, cell := range page {
				x := positions[i]
				if strings.TrimSpace(cell.Hanzi) != "" {
					b.WriteString(dialogueLine("Hanzi", pageStart, pageEnd, posText(x, row.HanziY, cell.Hanzi)))
					if highlightEnabled && cell.EndMS > cell.StartMS {
						activeStart, activeEnd, ok := clampWindow(cell.StartMS, cell.EndMS, window.StartMS, window.EndMS)
						if ok {
							b.WriteString(dialogueLine("HanziActive", formatASSTimestampMS(activeStart), formatASSTimestampMS(activeEnd), posText(x, row.HanziY, cell.Hanzi)))
						}
					}
				}
				if strings.TrimSpace(cell.Ruby) != "" {
					b.WriteString(dialogueLine("Ruby", pageStart, pageEnd, posText(x, row.RubyY, cell.Ruby)))
				}
			}
		}

		if hasEnglish {
			englishPages := splitEnglishPagesSynced(strings.TrimSpace(seg.EN), layout, len(pageWindows))
			englishRows := computeBottomSectionRows(layout, 1)
			for i, line := range englishPages {
				if i >= len(pageWindows) || len(englishRows) == 0 || strings.TrimSpace(line) == "" {
					break
				}
				pageStart := formatASSTimestampMS(pageWindows[i].StartMS)
				pageEnd := formatASSTimestampMS(pageWindows[i].EndMS)
				b.WriteString(dialogueLine("English", pageStart, pageEnd, posText(playW/2, englishRows[0], line)))
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
	HanziSpacing        float64
	RubySpacing         float64
	EnglishSpacing      float64
	PunctuationGapRatio float64
	MaxLineChars        int
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
	HighlightColor      string
	EnglishColor        string
	OutlineColor        string
	RubyFontName        string
	HanziFontName       string
	EnglishFontName     string
	RubyBold            int
	HanziBold           int
	EnglishBold         int
}

// 中文和日文公用一种布局计算方式，但预设参数不同
func newSubtitleLayout(playW, playH int, preset subtitlePreset) subtitleLayout {
	scale := float64(playH) / 1080.0
	topSectionOffset := int(float64(playH) * preset.TopSectionOffsetRatio)

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
	bottomSectionHeight := int(float64(boxHeight) * preset.BottomSectionRatio)
	// Keep the English area anchored to its original baseline. Using the
	// complement ratio preserves the previous position exactly despite
	// integer rounding.
	bottomSectionTop := boxTop + int(float64(boxHeight)*(1.0-preset.BottomSectionRatio))

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
		BaseGap:             maxInt(1, int(float64(preset.BaseGap)*scale)),
		HanziSpacing:        preset.HanziSpacing * scale,
		RubySpacing:         preset.RubySpacing * scale,
		EnglishSpacing:      preset.EnglishSpacing * scale,
		PunctuationGapRatio: preset.PunctuationGapRatio,
		MaxLineChars:        maxInt(1, preset.MaxLineChars),
		RowGap:              maxInt(1, int(float64(preset.RowGap)*scale)),
		TokenLineGap:        maxInt(6, int(float64(preset.TokenLineGap)*scale)),
		EnglishLineGap:      maxInt(4, int(float64(preset.EnglishLineGap)*scale)),
		TopSectionTop:       boxTop + int(float64(boxHeight)*preset.TopSectionTopInset) + topSectionOffset,
		TopSectionHeight:    topSectionHeight,
		BottomSectionTop:    bottomSectionTop + int(float64(boxHeight)*preset.BottomSectionTopInset),
		BottomSectionHeight: bottomSectionHeight,
		BoxColor:            preset.BoxColor,
		RubyColor:           preset.RubyColor,
		HanziColor:          preset.HanziColor,
		HighlightColor:      preset.HighlightColor,
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
	TopSectionOffsetRatio float64
	BottomSectionTopInset float64
	RubySize              int
	HanziSize             int
	EnglishSize           int
	BaseGap               int
	HanziSpacing          float64
	RubySpacing           float64
	EnglishSpacing        float64
	PunctuationGapRatio   float64
	MaxLineChars          int
	RowGap                int
	TokenLineGap          int
	EnglishLineGap        int
	BoxColor              string
	RubyColor             string
	HanziColor            string
	HighlightColor        string
	EnglishColor          string
	OutlineColor          string
	RubyBold              int
	HanziBold             int
	EnglishBold           int
}

func chineseSubtitlePresetFor(style int) subtitlePreset {
	switch style {
	case 2:
		return chineseSubtitlePresetStyle2()
	case 1:
		fallthrough
	default:
		return chineseSubtitlePresetStyle1()
	}
}

func chineseSubtitlePresetStyle1() subtitlePreset {
	preset := chineseSubtitlePresetStyle2()
	preset.BoxTopRatio = designType1TopBandTopRatio
	preset.BoxHeightRatio = designType1BoxHeightRatio
	preset.TopSectionRatio = designType1TopSectionRatio
	preset.BottomSectionRatio = designType1BottomSectionRatio
	preset.TopSectionTopInset = 0
	preset.TopSectionOffsetRatio = 0
	preset.BottomSectionTopInset = 0
	preset.RubyColor = assColorRGB(255, 255, 255)
	preset.HanziColor = assColorRGB(255, 255, 255)
	preset.HighlightColor = assColorRGB(196, 236, 121)
	preset.EnglishColor = assColorRGB(183, 236, 70)
	preset.OutlineColor = assColorRGB(0, 0, 0)
	applyChineseDesignType1Typography(&preset)
	return preset
}

func chineseSubtitlePresetStyle2() subtitlePreset {
	preset := subtitlePreset{
		RubyFontName:          "Tenor Sans",
		HanziFontName:         "HYWenRunSongYun J",
		EnglishFontName:       "Radley",
		BoxLeftRatio:          0,      // 字幕底框左边距占画面宽度比例
		BoxTopRatio:           0.5561, // 字幕底框顶部位置占画面高度比例
		BoxWidthRatio:         1,      // 字幕底框宽度占画面宽度比例
		BoxHeightRatio:        0.4029, // 字幕底框高度占画面高度比例
		TextWidthRatio:        0.94,   // 正文可用排版宽度占底框宽度比例
		TopSectionRatio:       0.7101, // 汉字区域高度占底框高度比例
		BottomSectionRatio:    0.2699, // 英文区域高度占底框高度比例
		TopSectionTopInset:    0.02,   // 上半区顶部额外内边距比例
		TopSectionOffsetRatio: 0.03,
		BottomSectionTopInset: 0,  // 下半区顶部额外内边距比例
		RubySize:              36, // 假名字号
		HanziSize:             76, // 中文正文字号
		EnglishSize:           46, // 英文字号
		BaseGap:               1,  // 字与字之间的基础间距
		HanziSpacing:          0.1083333333,
		RubySpacing:           -2,
		EnglishSpacing:        0, // 英文字距
		PunctuationGapRatio:   0.4615384615,
		MaxLineChars:          20, // 正文每行最大字符数
		RowGap:                1,
		TokenLineGap:          18,
		EnglishLineGap:        8,
		BoxColor:              "&HFF000000",
		RubyColor:             "&H00000000",
		HanziColor:            "&H00000000",
		HighlightColor:        "&H00CC66CC",
		EnglishColor:          assColorRGB(183, 236, 70),
		OutlineColor:          "&H00DDD6CF",
		RubyBold:              0,
		HanziBold:             1,
		EnglishBold:           1,
	}
	applyChineseDesignType1Typography(&preset)
	return preset
}

func applyChineseDesignType1Typography(preset *subtitlePreset) {
	preset.RubySize = 38
	preset.HanziSize = 78
	preset.EnglishSize = 49
	preset.RubyBold = 0
	preset.HanziBold = 1
	preset.EnglishBold = 1
}

// Keep a stable "base" entry for future style tuning workflows.
func chineseSubtitlePresetBase() subtitlePreset {
	return chineseSubtitlePresetStyle2()
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

func assSpacingText(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func assColorRGB(r, g, b int) string {
	return fmt.Sprintf("&H00%02X%02X%02X", b, g, r)
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
	b.WriteString("Style: Ruby," + layout.RubyFontName + "," + strconv.Itoa(layout.RubySize) + "," + layout.RubyColor + "," + layout.RubyColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.RubyBold) + ",0,0,0,100,100," + assSpacingText(layout.RubySpacing) + ",0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: Hanzi," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HanziColor + "," + layout.HanziColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.HanziBold) + ",0,0,0,100,100," + assSpacingText(layout.HanziSpacing) + ",0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: HanziActive," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HighlightColor + "," + layout.HighlightColor + "," + layout.OutlineColor + ",&H64000000,0,0,0,0,100,100," + assSpacingText(layout.HanziSpacing) + ",0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: English," + layout.EnglishFontName + "," + strconv.Itoa(layout.EnglishSize) + "," + layout.EnglishColor + "," + layout.EnglishColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.EnglishBold) + ",0,0,0,100,100," + assSpacingText(layout.EnglishSpacing) + ",0,1,1,0,5,10,10,10,1\n\n")

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
	for i := 0; i < len(tokens); i++ {
		if end, ok := inlineLatinWordTokenRun(tokens, i); ok {
			word := strings.Builder{}
			startMS := 0
			endMS := 0
			for j := i; j <= end; j++ {
				word.WriteString(strings.TrimSpace(tokens[j].Char))
				if startMS == 0 || (tokens[j].StartMS > 0 && tokens[j].StartMS < startMS) {
					startMS = tokens[j].StartMS
				}
				if tokens[j].EndMS > endMS {
					endMS = tokens[j].EndMS
				}
			}
			text := word.String()
			gap := latinCellGapAfter(tokens, end, layout)
			out = append(out, tokenCell{
				Hanzi:   text,
				Ruby:    "",
				Width:   estimateTextWidth(text, float64(layout.HanziSize), false),
				Gap:     gap,
				StartMS: startMS,
				EndMS:   endMS,
			})
			i = end
			continue
		}
		tk := tokens[i]
		if isWhitespaceOnlyText(tk.Char) {
			out = append(out, tokenCell{
				Hanzi:   tk.Char,
				Ruby:    "",
				Width:   estimateWhitespaceWidth(tk.Char, float64(layout.HanziSize)),
				Gap:     0,
				StartMS: tk.StartMS,
				EndMS:   tk.EndMS,
			})
			continue
		}
		hanzi := strings.TrimSpace(tk.Char)
		ruby := strings.TrimSpace(tk.Reading)
		if hanzi == "" && ruby == "" {
			continue
		}
		hanziW := estimateTextWidth(hanzi, float64(layout.HanziSize), true)
		rubyW := estimateTextWidth(ruby, float64(layout.RubySize), false)
		w := maxFloat(hanziW, rubyW)
		if isPunctuationText(hanzi) {
			w = maxFloat(w*0.60, float64(layout.HanziSize)*0.48)
		}
		gap := layout.HanziSpacing
		if isPunctuationText(hanzi) {
			gap = layout.HanziSpacing * layout.PunctuationGapRatio
		}
		if isQuotePunctuationText(hanzi) {
			w = maxFloat(estimateTextWidth(hanzi, float64(layout.HanziSize), false)*0.55, float64(layout.HanziSize)*0.18)
			if quoteTouchesInlineLatin(tokens, out, i) {
				gap = 0
			}
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

// Main subtitle pages are display-only splits. We keep natural long segments
// intact in the audio/script, but prefer punctuation boundaries on screen and
// cap each page to at most 20 visible characters.
func paginateTokenCells(cells []tokenCell, layout subtitleLayout) [][]tokenCell {
	if len(cells) == 0 {
		return nil
	}
	pages := make([][]tokenCell, 0, 2)
	for start := 0; start < len(cells); {
		end := chooseChinesePageBreak(cells, start, layout)
		if end <= start {
			end = start + 1
		}
		pages = append(pages, append([]tokenCell(nil), cells[start:end]...))
		start = end
	}
	return pages
}

func chooseChinesePageBreak(cells []tokenCell, start int, layout subtitleLayout) int {
	if start >= len(cells) {
		return start
	}
	charCount := 0
	width := 0.0
	limit := float64(layout.MaxTextWidth)
	charLimit := subtitlePageCharLimit(layout)
	bestEnd := start
	bestPunctEnd := -1
	forcedWrappedBlockBreak := -1

	for i := start; i < len(cells); i++ {
		unitChars := chineseCellPageUnits(cells[i])
		if unitChars <= 0 && !isWhitespaceOnlyText(cells[i].Hanzi) {
			unitChars = 1
		}
		nextWidth := width
		if i > start {
			nextWidth += cells[i-1].Gap
		}
		nextWidth += cells[i].Width
		if i > start && (charCount+unitChars > charLimit || nextWidth > limit) {
			break
		}

		charCount += unitChars
		width = nextWidth
		bestEnd = i + 1
		if subtitleEndsWithPunctuation(cells[i].Hanzi) && !isOpeningWrapperText(cells[i].Hanzi) {
			bestPunctEnd = i + 1
			if i > start && isBoundarySymbolText(cells[i].Hanzi) && longWrappedSpanStartsAfter(cells, i, charLimit, limit) {
				forcedWrappedBlockBreak = i + 1
				break
			}
		}
		if charCount >= charLimit {
			break
		}
	}
	if forcedWrappedBlockBreak > start {
		return forcedWrappedBlockBreak
	}
	if bestEnd == start {
		return start + 1
	}
	candidateEnd := bestEnd
	if bestPunctEnd > start {
		candidateEnd = bestPunctEnd
	}
	texts := make([]string, len(cells))
	for i := range cells {
		texts[i] = cells[i].Hanzi
	}
	adjusted := adjustSubtitlePageBreak(texts, start, candidateEnd)
	if adjusted > candidateEnd {
		units, width := chinesePageMetrics(cells, start, adjusted)
		if units > charLimit || width > limit {
			return candidateEnd
		}
	}
	return adjusted
}

func longWrappedSpanStartsAfter(cells []tokenCell, breakAt int, charLimit int, widthLimit float64) bool {
	next := nextNonWhitespaceCellIndex(cells, breakAt+1)
	if next < 0 || !isOpeningWrapperText(cells[next].Hanzi) {
		return false
	}
	end, ok := findWrappedSpanEnd(cells, next)
	if !ok || end <= next {
		return false
	}
	units, width := chinesePageMetrics(cells, next, end+1)
	return units > charLimit || width > widthLimit
}

func nextNonWhitespaceCellIndex(cells []tokenCell, start int) int {
	for i := start; i < len(cells); i++ {
		if !isWhitespaceOnlyText(cells[i].Hanzi) {
			return i
		}
	}
	return -1
}

func findWrappedSpanEnd(cells []tokenCell, openIndex int) (int, bool) {
	if openIndex < 0 || openIndex >= len(cells) {
		return 0, false
	}
	open := strings.TrimSpace(cells[openIndex].Hanzi)
	if open == "" {
		return 0, false
	}
	close, symmetric, ok := pairedClosingWrapper(open)
	if !ok {
		return 0, false
	}
	depth := 1
	for i := openIndex + 1; i < len(cells); i++ {
		current := strings.TrimSpace(cells[i].Hanzi)
		if current == "" {
			continue
		}
		if symmetric {
			if current == close {
				depth--
				if depth == 0 {
					return i, true
				}
			}
			continue
		}
		if current == open {
			depth++
			continue
		}
		if current == close {
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func pairedClosingWrapper(open string) (string, bool, bool) {
	switch open {
	case "'", "\"":
		return open, true, true
	case "‘":
		return "’", false, true
	case "“":
		return "”", false, true
	case "「":
		return "」", false, true
	case "『":
		return "』", false, true
	case "(":
		return ")", false, true
	case "（":
		return "）", false, true
	case "[":
		return "]", false, true
	case "【":
		return "】", false, true
	case "《":
		return "》", false, true
	case "〈":
		return "〉", false, true
	default:
		return "", false, false
	}
}

func chinesePageMetrics(cells []tokenCell, start, end int) (int, float64) {
	if start < 0 {
		start = 0
	}
	if end > len(cells) {
		end = len(cells)
	}
	if end <= start {
		return 0, 0
	}
	units := 0
	width := 0.0
	for i := start; i < end; i++ {
		units += chineseCellPageUnits(cells[i])
		if i > start {
			width += cells[i-1].Gap
		}
		width += cells[i].Width
	}
	return units, width
}

func chinesePageStartTimes(pages [][]tokenCell) []int {
	out := make([]int, 0, len(pages))
	for _, page := range pages {
		start := 0
		for _, cell := range page {
			if cell.StartMS > 0 {
				start = cell.StartMS
				break
			}
		}
		out = append(out, start)
	}
	return out
}

func chinesePageHasRuby(page []tokenCell) bool {
	for _, cell := range page {
		if strings.TrimSpace(cell.Ruby) != "" {
			return true
		}
	}
	return false
}

func chinesePageWeights(pages [][]tokenCell) []int {
	out := make([]int, 0, len(pages))
	for _, page := range pages {
		weight := 0
		for _, cell := range page {
			weight += chineseCellPageUnits(cell)
		}
		out = append(out, maxInt(1, weight))
	}
	return out
}

func splitEnglishPagesSynced(text string, layout subtitleLayout, pageCount int) []string {
	text = normalizeEnglishSubtitleSpacing(text)
	if text == "" {
		return nil
	}
	if pageCount <= 1 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	limit := float64(layout.MaxTextWidth) * 0.96
	pages := make([]string, 0, pageCount)
	index := 0
	for page := 0; page < pageCount && index < len(words); page++ {
		remainingPages := pageCount - page
		remainingWords := len(words) - index
		targetWords := (remainingWords + remainingPages - 1) / remainingPages

		current := ""
		lastGood := ""
		lastGoodIndex := index
		bestPunct := ""
		bestPunctIndex := index
		wordsUsed := 0

		for j := index; j < len(words); j++ {
			candidate := words[j]
			if current != "" {
				candidate = current + " " + words[j]
			}
			if current != "" && estimateTextWidth(candidate, float64(layout.EnglishSize), false) > limit {
				break
			}
			current = candidate
			wordsUsed++
			lastGood = current
			lastGoodIndex = j + 1
			if subtitleEndsWithPunctuation(words[j]) {
				bestPunct = current
				bestPunctIndex = j + 1
			}
			if wordsUsed >= targetWords && bestPunct != "" {
				break
			}
			if len(words)-(j+1) < remainingPages-1 {
				break
			}
		}

		if bestPunct != "" && lastGood != "" && lastGoodIndex-bestPunctIndex <= 3 {
			pages = append(pages, normalizeEnglishSubtitleSpacing(bestPunct))
			index = bestPunctIndex
			continue
		}
		if lastGood == "" {
			lastGood = words[index]
			lastGoodIndex = index + 1
		}
		pages = append(pages, normalizeEnglishSubtitleSpacing(lastGood))
		index = lastGoodIndex
	}
	return pages
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
	text = normalizeEnglishSubtitleSpacing(text)
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
		lines = append(lines, normalizeEnglishSubtitleSpacing(current))
	}
	if len(lines) > maxLines {
		tail := normalizeEnglishSubtitleSpacing(strings.Join(lines[maxLines-1:], " "))
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

func estimateWhitespaceWidth(text string, fontSize float64) float64 {
	width := 0.0
	for _, r := range []rune(text) {
		if unicode.IsSpace(r) {
			width += fontSize * 0.12
		}
	}
	return width
}

func normalizeEnglishSubtitleSpacing(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		" ,", ",",
		" .", ".",
		" !", "!",
		" ?", "?",
		" ;", ";",
		" :", ":",
		" )", ")",
		" ]", "]",
		" }", "}",
		" ’", "’",
		" ”", "”",
		"( ", "(",
		"[ ", "[",
		"{ ", "{",
		"‘ ", "‘",
		"“ ", "“",
		" '", "'",
		"' ", "'",
		" \"", "\"",
		"\" ", "\"",
	)
	return replacer.Replace(text)
}

func hasAnyRuby(tokens []dto.PodcastToken) bool {
	for _, t := range tokens {
		if strings.TrimSpace(t.Reading) != "" {
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

func isWhitespaceOnlyText(s string) bool {
	return s != "" && strings.TrimSpace(s) == ""
}

func isPunctuationRune(r rune) bool {
	return unicode.IsPunct(r) || strings.ContainsRune("，。！？；：“”‘’（）《》、…", r)
}

func isInlineEnglishText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	hasAlphaNum := false
	for _, r := range []rune(trimmed) {
		switch {
		case unicode.In(unicode.ToLower(r), unicode.Latin), unicode.IsDigit(r):
			hasAlphaNum = true
		case r == '-' || r == '\'':
			continue
		default:
			return false
		}
	}
	return hasAlphaNum
}

func latinCellGapAfter(tokens []dto.PodcastToken, end int, layout subtitleLayout) float64 {
	if end < 0 || end >= len(tokens)-1 {
		return layout.HanziSpacing
	}
	nextRaw := tokens[end+1].Char
	if isWhitespaceOnlyText(nextRaw) {
		return 0
	}
	if isQuotePunctuationText(nextRaw) {
		return 0
	}
	if isLatinWordConnectorToken(nextRaw) {
		return 0
	}
	if isLatinWordBodyToken(nextRaw) {
		return estimateWhitespaceWidth(" ", float64(layout.HanziSize))
	}
	if hasTrailingWhitespace(tokens[end].Char) {
		return estimateWhitespaceWidth(" ", float64(layout.HanziSize))
	}
	return layout.HanziSpacing
}

func quoteTouchesInlineLatin(tokens []dto.PodcastToken, built []tokenCell, idx int) bool {
	prevInline := len(built) > 0 && isInlineEnglishText(built[len(built)-1].Hanzi)
	if prevInline {
		return true
	}
	if idx+1 >= len(tokens) {
		return false
	}
	_, ok := inlineLatinWordTokenRun(tokens, idx+1)
	return ok
}

func isQuotePunctuationText(s string) bool {
	switch strings.TrimSpace(s) {
	case "'", "\"", "‘", "’", "“", "”", "「", "」", "『", "』":
		return true
	default:
		return false
	}
}

func isOpeningWrapperText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	_, _, ok := pairedClosingWrapper(trimmed)
	return ok
}

func isBoundarySymbolText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	rs := []rune(trimmed)
	if len(rs) != 1 {
		return false
	}
	if isOpeningWrapperText(trimmed) {
		return false
	}
	return isPunctuationRune(rs[0])
}

func hasTrailingWhitespace(s string) bool {
	rs := []rune(s)
	if len(rs) == 0 {
		return false
	}
	return unicode.IsSpace(rs[len(rs)-1])
}

func chineseCellPageUnits(cell tokenCell) int {
	switch {
	case isWhitespaceOnlyText(cell.Hanzi):
		return 0
	case isInlineEnglishText(cell.Hanzi):
		return 1
	default:
		return subtitleRuneCount(cell.Hanzi)
	}
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

func minInt(a, b int) int {
	if a < b {
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
