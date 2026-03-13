package podcast

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"worker/internal/dto"
)

func writeJapaneseASS(script dto.PodcastScript, projectDir, resolution string, style int) (string, error) {
	if len(script.Segments) == 0 {
		return "", nil
	}

	playW, playH := resolutionSize(resolution)
	layout := japaneseSubtitleLayout(playW, playH, style)

	var b strings.Builder
	writeJapaneseASSHeader(&b, layout)
	for _, seg := range script.Segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		cells := buildJapaneseCharCells(seg, layout)
		if len(cells) == 0 && strings.TrimSpace(seg.EN) == "" {
			continue
		}

		start := formatASSTimestampMS(seg.StartMS)
		end := formatASSTimestampMS(seg.EndMS)
		rows := computeJapaneseRows(layout, lineCountForJapaneseCells(cells))

		b.WriteString(dialogueLine("Box", start, end, boxText(layout)))
		for _, cell := range cells {
			if cell.Line >= len(rows) {
				continue
			}
			row := rows[cell.Line]
			b.WriteString(dialogueLine("JaBase", start, end, posText(cell.CenterX, row.BaseY, cell.Char)))
			if cell.EndMS > cell.StartMS {
				b.WriteString(dialogueLine("JaActive", formatASSTimestampMS(cell.StartMS), formatASSTimestampMS(cell.EndMS), posText(cell.CenterX, row.BaseY, cell.Char)))
			}
		}

		rubySpans := seg.RubySpans
		if len(rubySpans) == 0 && len(seg.RubyTokens) > 0 {
			rubySpans = buildJapaneseRubySpans(seg)
		}
		for _, span := range rubySpans {
			ruby := strings.TrimSpace(span.Ruby)
			if ruby == "" {
				continue
			}
			centerX, line, ok := japaneseRubyCenter(span, cells)
			if !ok || line >= len(rows) {
				continue
			}
			b.WriteString(dialogueLine("Ruby", start, end, posText(centerX, rows[line].RubyY, ruby)))
		}

		if english := strings.TrimSpace(seg.EN); english != "" {
			lines := splitEnglishLines(english, layout, 2)
			englishRows := computeBottomSectionRows(layout, len(lines))
			for i, line := range lines {
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

type japaneseSubtitleRow struct {
	RubyY int
	BaseY int
}

type japaneseCharCell struct {
	Index   int
	Char    string
	CenterX int
	Width   float64
	Line    int
	StartMS int
	EndMS   int
}

type japaneseTokenGroup struct {
	Cells []tokenCell
}

func buildJapaneseCharCells(seg dto.PodcastSegment, layout subtitleLayout) []japaneseCharCell {
	chars := japaneseSegmentChars(seg)
	if len(chars) == 0 {
		return nil
	}

	tokenCells := make([]tokenCell, 0, len(chars))
	for _, ch := range chars {
		trimmed := strings.TrimSpace(ch.Char)
		if trimmed == "" {
			trimmed = ch.Char
		}
		w := estimateTextWidth(trimmed, float64(layout.HanziSize), true)
		if isPunctuationText(trimmed) {
			w = maxFloat(w*0.60, float64(layout.HanziSize)*0.48)
		}
		tokenCells = append(tokenCells, tokenCell{
			Hanzi: trimmed,
			Width: w,
			Gap:   japaneseCharGap(trimmed, layout),
		})
	}

	lines := splitJapaneseTokenLines(seg, tokenCells, layout.MaxTextWidth, 2)
	if len(lines) == 0 {
		return nil
	}

	out := make([]japaneseCharCell, 0, len(chars))
	charIndex := 0
	for lineIdx, line := range lines {
		centers := computeCellCenters(line, layout)
		for i := range line {
			if charIndex >= len(chars) || i >= len(centers) {
				break
			}
			out = append(out, japaneseCharCell{
				Index:   chars[charIndex].Index,
				Char:    chars[charIndex].Char,
				CenterX: centers[i],
				Width:   line[i].Width,
				Line:    lineIdx,
				StartMS: chars[charIndex].StartMS,
				EndMS:   chars[charIndex].EndMS,
			})
			charIndex++
		}
	}
	return out
}

func splitJapaneseTokenLines(seg dto.PodcastSegment, cells []tokenCell, maxWidth int, maxLines int) [][]tokenCell {
	if len(cells) == 0 {
		return nil
	}
	groups := buildJapaneseTokenGroups(seg, cells)
	if len(groups) == 0 {
		return splitTokenLines(cells, maxWidth, maxLines)
	}
	if maxLines == 2 {
		if balanced := splitJapaneseTokenLinesBalanced(groups, maxWidth); len(balanced) > 0 {
			return balanced
		}
	}

	lines := make([][]tokenCell, 0, maxLines)
	current := make([]tokenCell, 0, len(cells))
	currentWidth := 0.0
	limit := float64(maxWidth)

	for _, group := range groups {
		if len(group.Cells) == 0 {
			continue
		}
		groupWidth := japaneseTokenGroupWidth(group.Cells)
		nextWidth := currentWidth
		if len(current) > 0 {
			nextWidth += current[len(current)-1].Gap
		}
		nextWidth += groupWidth

		if len(current) > 0 && nextWidth > limit && len(lines) < maxLines-1 {
			lines = append(lines, current)
			current = append([]tokenCell(nil), group.Cells...)
			currentWidth = groupWidth
			continue
		}

		if len(current) > 0 {
			currentWidth += current[len(current)-1].Gap
		}
		current = append(current, group.Cells...)
		currentWidth += groupWidth
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

func splitJapaneseTokenLinesBalanced(groups []japaneseTokenGroup, maxWidth int) [][]tokenCell {
	if len(groups) == 0 {
		return nil
	}
	all := flattenJapaneseTokenGroups(groups)
	if japaneseLineWidth(all) <= float64(maxWidth) {
		return [][]tokenCell{all}
	}
	if len(groups) < 2 {
		return nil
	}

	limit := float64(maxWidth)
	bestIdx := -1
	bestScore := 0.0
	for i := 1; i < len(groups); i++ {
		left := flattenJapaneseTokenGroups(groups[:i])
		right := flattenJapaneseTokenGroups(groups[i:])
		leftWidth := japaneseLineWidth(left)
		rightWidth := japaneseLineWidth(right)
		if leftWidth > limit || rightWidth > limit {
			continue
		}
		score := absFloat(leftWidth - rightWidth)
		if bestIdx == -1 || score < bestScore {
			bestIdx = i
			bestScore = score
		}
	}
	if bestIdx == -1 {
		return nil
	}
	return [][]tokenCell{
		flattenJapaneseTokenGroups(groups[:bestIdx]),
		flattenJapaneseTokenGroups(groups[bestIdx:]),
	}
}

func buildJapaneseTokenGroups(seg dto.PodcastSegment, cells []tokenCell) []japaneseTokenGroup {
	if len(cells) == 0 {
		return nil
	}
	rubySpans := seg.RubySpans
	if len(rubySpans) == 0 && len(seg.RubyTokens) > 0 {
		rubySpans = buildJapaneseRubySpans(seg)
	}
	if len(rubySpans) == 0 {
		out := make([]japaneseTokenGroup, 0, len(cells))
		for _, cell := range cells {
			out = append(out, japaneseTokenGroup{Cells: []tokenCell{cell}})
		}
		return out
	}

	out := make([]japaneseTokenGroup, 0, len(cells))
	spanIndex := 0
	for i := 0; i < len(cells); {
		if spanIndex < len(rubySpans) && i == rubySpans[spanIndex].StartIndex {
			end := rubySpans[spanIndex].EndIndex
			if end >= len(cells) {
				end = len(cells) - 1
			}
			if end >= i {
				groupCells := append([]tokenCell(nil), cells[i:end+1]...)
				out = append(out, japaneseTokenGroup{Cells: groupCells})
				i = end + 1
				spanIndex++
				continue
			}
			spanIndex++
		}
		out = append(out, japaneseTokenGroup{Cells: []tokenCell{cells[i]}})
		i++
	}
	return out
}

func japaneseTokenGroupWidth(cells []tokenCell) float64 {
	return japaneseLineWidth(cells)
}

func flattenJapaneseTokenGroups(groups []japaneseTokenGroup) []tokenCell {
	total := 0
	for _, group := range groups {
		total += len(group.Cells)
	}
	out := make([]tokenCell, 0, total)
	for _, group := range groups {
		out = append(out, group.Cells...)
	}
	return out
}

func japaneseLineWidth(cells []tokenCell) float64 {
	total := 0.0
	for i, cell := range cells {
		total += cell.Width
		if i != len(cells)-1 {
			total += cell.Gap
		}
	}
	return total
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func japaneseSegmentChars(seg dto.PodcastSegment) []dto.PodcastCharToken {
	if len(seg.Chars) > 0 {
		return seg.Chars
	}
	text := strings.TrimSpace(seg.DisplayJA)
	if text == "" {
		text = strings.TrimSpace(seg.JA)
	}
	if text == "" {
		text = strings.TrimSpace(seg.ZH)
	}
	runes := []rune(text)
	out := make([]dto.PodcastCharToken, 0, len(runes))
	for i, r := range runes {
		out = append(out, dto.PodcastCharToken{
			Index: i,
			Char:  string(r),
		})
	}
	return out
}

func japaneseCharGap(char string, layout subtitleLayout) float64 {
	if isPunctuationText(char) {
		return float64(layout.BaseGap) * 0.30
	}
	return float64(layout.BaseGap) * 0.65
}

func lineCountForJapaneseCells(cells []japaneseCharCell) int {
	count := 0
	for _, cell := range cells {
		if cell.Line+1 > count {
			count = cell.Line + 1
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func computeJapaneseRows(layout subtitleLayout, lineCount int) []japaneseSubtitleRow {
	baseRows := computeTopSectionRows(layout, lineCount, true)
	rows := make([]japaneseSubtitleRow, 0, len(baseRows))
	for _, row := range baseRows {
		rows = append(rows, japaneseSubtitleRow{
			RubyY: row.RubyY,
			BaseY: row.HanziY,
		})
	}
	return rows
}

func japaneseRubyCenter(span dto.PodcastRubySpan, cells []japaneseCharCell) (int, int, bool) {
	var left, right float64
	found := false
	line := 0
	for _, cell := range cells {
		if cell.Index < span.StartIndex || cell.Index > span.EndIndex {
			continue
		}
		cellLeft := float64(cell.CenterX) - cell.Width/2
		cellRight := float64(cell.CenterX) + cell.Width/2
		if !found || cellLeft < left {
			left = cellLeft
		}
		if !found || cellRight > right {
			right = cellRight
		}
		line = cell.Line
		found = true
	}
	if !found {
		return 0, 0, false
	}
	return int((left + right) / 2), line, true
}

func writeJapaneseASSHeader(b *strings.Builder, layout subtitleLayout) {
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
	b.WriteString("Style: Box," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.BoxColor + "," + layout.BoxColor + "," + layout.BoxColor + "," + layout.BoxColor + ",0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n")
	b.WriteString("Style: Ruby," + layout.RubyFontName + "," + strconv.Itoa(layout.RubySize) + "," + layout.RubyColor + "," + layout.RubyColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.RubyBold) + ",0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: JaBase," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HanziColor + "," + layout.HanziColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.HanziBold) + ",0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: JaActive," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + ",&H0078ECFF,&H0078ECFF," + layout.OutlineColor + ",&H64000000,0,0,0,0,100,100,0,0,1,1,0,5,10,10,10,1\n")
	b.WriteString("Style: English," + layout.EnglishFontName + "," + strconv.Itoa(layout.EnglishSize) + "," + layout.EnglishColor + "," + layout.EnglishColor + "," + layout.EnglishColor + ",&H00000000," + strconv.Itoa(layout.EnglishBold) + ",0,0,0,100,100,0,0,1,0,0,5,10,10,10,1\n\n")

	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n")
}

func japaneseSubtitleLayout(playW, playH, style int) subtitleLayout {
	return newSubtitleLayout(playW, playH, japaneseSubtitlePresetFor(style))
}

func japaneseSubtitlePresetFor(style int) subtitlePreset {
	switch style {
	case 2:
		return subtitlePreset{
			RubyFontName:          "Maruko Gothic CJKjp Light", // 假名字体
			HanziFontName:         "Hina Mincho", // 日文正文汉字/假名字体
			EnglishFontName:       "Radley",      // 英文字幕字体
			BoxLeftRatio:          0,             // 字幕底框左边距占画面宽度比例
			BoxTopRatio:           0.415,         // 字幕底框顶部位置占画面高度比例
			BoxWidthRatio:         1,             // 字幕底框宽度占画面宽度比例
			BoxHeightRatio:        0.535,         // 字幕底框高度占画面高度比例
			TextWidthRatio:        0.88,          // 正文可用排版宽度占底框宽度比例
			TopSectionRatio:       0.748,         // 上半区（日文+假名）占底框高度比例
			BottomSectionRatio:    0.252,         // 下半区（英文）占底框高度比例
			TopSectionTopInset:    0,             // 上半区顶部额外内边距比例
			BottomSectionTopInset: 0,             // 下半区顶部额外内边距比例
			RubySize:              42,            // 假名字号
			HanziSize:             70,            // 日文正文字号
			EnglishSize:           44,            // 英文字号
			BaseGap:               4,             // 字与字之间的基础间距
			RowGap:                10,            // 假名与正文之间的垂直间距
			TokenLineGap:          16,            // 两行日文之间的垂直间距
			EnglishLineGap:        10,            // 英文多行时的行间距
			BoxColor:              "&HFF000000", // 底框颜色
			RubyColor:             "&H00383838", // 假名颜色
			HanziColor:            "&H00383838", // 日文正文颜色
			EnglishColor:          "&H005C2C10", // 英文字幕颜色
			OutlineColor:          "&H00303030", // 轮廓/描边颜色
			RubyBold:              0,             // 假名是否粗体
			HanziBold:             0,             // 日文正文是否粗体
			EnglishBold:           0,             // 英文字幕是否粗体
		}
	case 1:
		fallthrough
	default:
		return subtitlePreset{
			RubyFontName:          "Maruko Gothic CJKjp Light", // 假名字体
			HanziFontName:         "Hina Mincho", // 日文正文汉字/假名字体
			EnglishFontName:       "Radley",      // 英文字幕字体
			BoxLeftRatio:          0,             // 字幕底框左边距占画面宽度比例
			BoxTopRatio:           0.56,          // 字幕底框顶部位置占画面高度比例
			BoxWidthRatio:         1,             // 字幕底框宽度占画面宽度比例
			BoxHeightRatio:        0.40,          // 字幕底框高度占画面高度比例
			TextWidthRatio:        0.87,          // 正文可用排版宽度占底框宽度比例
			TopSectionRatio:       0.60,          // 上半区（日文+假名）占底框高度比例
			BottomSectionRatio:    0.24,          // 下半区（英文）占底框高度比例
			TopSectionTopInset:    0.06,          // 上半区顶部额外内边距比例
			BottomSectionTopInset: 0.02,          // 下半区顶部额外内边距比例
			RubySize:              42,            // 假名字号
			HanziSize:             70,            // 日文正文字号
			EnglishSize:           44,            // 英文字号
			BaseGap:               4,             // 字与字之间的基础间距
			RowGap:                8,             // 假名与正文之间的垂直间距
			TokenLineGap:          16,            // 两行日文之间的垂直间距
			EnglishLineGap:        8,             // 英文多行时的行间距
			BoxColor:              "&HFF000000", // 底框颜色
			RubyColor:             "&H00383838", // 假名颜色
			HanziColor:            "&H00383838", // 日文正文颜色
			EnglishColor:          "&H005C2C10", // 英文字幕颜色
			OutlineColor:          "&H002A3640", // 轮廓/描边颜色
			RubyBold:              0,             // 假名是否粗体
			HanziBold:             0,             // 日文正文是否粗体
			EnglishBold:           0,             // 英文字幕是否粗体
		}
	}
}

func buildJapaneseRubySpans(seg dto.PodcastSegment) []dto.PodcastRubySpan {
	text := strings.TrimSpace(seg.DisplayJA)
	if text == "" {
		text = strings.TrimSpace(seg.JA)
	}
	if text == "" || len(seg.RubyTokens) == 0 {
		return nil
	}
	runes := []rune(text)
	out := make([]dto.PodcastRubySpan, 0, len(seg.RubyTokens))
	searchFrom := 0
	for _, token := range seg.RubyTokens {
		surface := strings.TrimSpace(token.Surface)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		start, end, ok := findJapaneseRubySurfaceRange(runes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeJapaneseRubySpanRange(runes, dto.PodcastRubySpan{
			StartIndex: start,
			EndIndex:   end,
			Ruby:       reading,
		})
		if !ok {
			searchFrom = end + 1
			continue
		}
		out = append(out, span)
		searchFrom = end + 1
	}
	return dedupeJapaneseRubySpans(out)
}

func findJapaneseRubySurfaceRange(textRunes, surfaceRunes []rune, searchFrom int) (int, int, bool) {
	if len(surfaceRunes) == 0 || len(textRunes) == 0 || searchFrom >= len(textRunes) {
		return 0, 0, false
	}
	maxStart := len(textRunes) - len(surfaceRunes)
	for start := maxInt(0, searchFrom); start <= maxStart; start++ {
		match := true
		for i := range surfaceRunes {
			if textRunes[start+i] != surfaceRunes[i] {
				match = false
				break
			}
		}
		if match {
			return start, start + len(surfaceRunes) - 1, true
		}
	}
	return 0, 0, false
}

func normalizeJapaneseRubySpanRange(runes []rune, span dto.PodcastRubySpan) (dto.PodcastRubySpan, bool) {
	firstHan := -1
	lastHan := -1
	for i := span.StartIndex; i <= span.EndIndex; i++ {
		if unicode.In(runes[i], unicode.Han) {
			if firstHan == -1 {
				firstHan = i
			}
			lastHan = i
		}
	}
	if firstHan == -1 {
		return dto.PodcastRubySpan{}, false
	}
	for firstHan > 0 && unicode.In(runes[firstHan-1], unicode.Han) {
		firstHan--
	}
	for lastHan+1 < len(runes) && unicode.In(runes[lastHan+1], unicode.Han) {
		lastHan++
	}
	span.StartIndex = firstHan
	span.EndIndex = lastHan
	return span, true
}

func dedupeJapaneseRubySpans(spans []dto.PodcastRubySpan) []dto.PodcastRubySpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]dto.PodcastRubySpan, 0, len(spans))
	lastEnd := -1
	for _, span := range spans {
		if span.StartIndex <= lastEnd {
			continue
		}
		out = append(out, span)
		lastEnd = span.EndIndex
	}
	return out
}
