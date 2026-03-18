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
		pages := paginateJapaneseCellPages(seg, cells, layout)
		pageWindows := buildSubtitlePageWindows(seg.StartMS, seg.EndMS, japanesePageStartTimes(pages))

		b.WriteString(dialogueLine("Box", start, end, boxText(layout)))
		for pageIndex, page := range pages {
			if pageIndex >= len(pageWindows) || len(page) == 0 {
				break
			}
			rows := computeJapaneseRows(layout, 1)
			if len(rows) == 0 {
				continue
			}
			row := rows[0]
			window := pageWindows[pageIndex]
			pageStart := formatASSTimestampMS(window.StartMS)
			pageEnd := formatASSTimestampMS(window.EndMS)
			for _, cell := range page {
				b.WriteString(dialogueLine("JaBase", pageStart, pageEnd, posText(cell.CenterX, row.BaseY, cell.Char)))
				if cell.EndMS > cell.StartMS {
					b.WriteString(dialogueLine("JaActive", formatASSTimestampMS(cell.StartMS), formatASSTimestampMS(cell.EndMS), posText(cell.CenterX, row.BaseY, cell.Char)))
				}
			}

			tokenSpans := seg.TokenSpans
			if len(tokenSpans) == 0 && len(seg.Tokens) > 0 {
				tokenSpans = buildJapaneseTokenSpans(seg)
			}
			for _, span := range tokenSpans {
				ruby := strings.TrimSpace(span.Reading)
				if ruby == "" {
					continue
				}
				centerX, line, ok := japaneseRubyCenter(span, page)
				if !ok || line >= len(rows) {
					continue
				}
				b.WriteString(dialogueLine("Ruby", pageStart, pageEnd, posText(centerX, rows[line].RubyY, ruby)))
			}
		}

		if english := strings.TrimSpace(seg.EN); english != "" {
			englishPages := splitEnglishPagesSynced(english, layout, len(pageWindows))
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

type japaneseSubtitleRow struct {
	RubyY int
	BaseY int
}

type japaneseCharCell struct {
	StartIndex int
	EndIndex   int
	Char       string
	CenterX    int
	Width      float64
	Gap        float64
	Line       int
	StartMS    int
	EndMS      int
}

type japaneseTokenGroup struct {
	Cells []japaneseCharCell
}

func buildJapaneseCharCells(seg dto.PodcastSegment, layout subtitleLayout) []japaneseCharCell {
	tokens := japaneseSegmentTokens(seg)
	if len(tokens) == 0 {
		return nil
	}
	return buildJapaneseLayoutCells(tokens, layout)
}

func buildJapaneseLayoutCells(tokens []dto.PodcastToken, layout subtitleLayout) []japaneseCharCell {
	out := make([]japaneseCharCell, 0, len(tokens))
	for i := 0; i < len(tokens); {
		out = append(out, japaneseCharCell{
			StartIndex: i,
			EndIndex:   i,
			Char:       tokens[i].Text,
			Width:      estimateJapaneseCellWidth(tokens[i].Text, layout),
			Gap:        japaneseCharGap(tokens[i].Text, layout),
			StartMS:    tokens[i].StartMS,
			EndMS:      tokens[i].EndMS,
		})
		i++
	}
	return out
}

func splitJapaneseTokenLines(seg dto.PodcastSegment, cells []japaneseCharCell, maxWidth int, maxLines int) [][]japaneseCharCell {
	if len(cells) == 0 {
		return nil
	}
	groups := buildJapaneseTokenGroups(seg, cells)
	if len(groups) == 0 {
		return splitJapaneseCells(cells, maxWidth, maxLines)
	}
	if maxLines == 2 {
		if balanced := splitJapaneseTokenLinesBalanced(groups, maxWidth); len(balanced) > 0 {
			return balanced
		}
	}

	lines := make([][]japaneseCharCell, 0, maxLines)
	current := make([]japaneseCharCell, 0, len(cells))
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
			current = append([]japaneseCharCell(nil), group.Cells...)
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
		tail := make([]japaneseCharCell, 0)
		for _, extra := range lines[maxLines-1:] {
			tail = append(tail, extra...)
		}
		lines = append(lines[:maxLines-1], tail)
	}
	return lines
}

func splitJapaneseTokenLinesBalanced(groups []japaneseTokenGroup, maxWidth int) [][]japaneseCharCell {
	if len(groups) == 0 {
		return nil
	}
	all := flattenJapaneseTokenGroups(groups)
	if japaneseLineWidth(all) <= float64(maxWidth) {
		return [][]japaneseCharCell{all}
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
	return [][]japaneseCharCell{
		flattenJapaneseTokenGroups(groups[:bestIdx]),
		flattenJapaneseTokenGroups(groups[bestIdx:]),
	}
}

func splitJapaneseCells(cells []japaneseCharCell, maxWidth int, maxLines int) [][]japaneseCharCell {
	if len(cells) == 0 {
		return nil
	}
	lines := make([][]japaneseCharCell, 0, maxLines)
	current := make([]japaneseCharCell, 0, len(cells))
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
			current = []japaneseCharCell{cell}
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
		tail := make([]japaneseCharCell, 0)
		for _, extra := range lines[maxLines-1:] {
			tail = append(tail, extra...)
		}
		lines = append(lines[:maxLines-1], tail)
	}
	return lines
}

func paginateJapaneseCellPages(seg dto.PodcastSegment, cells []japaneseCharCell, layout subtitleLayout) [][]japaneseCharCell {
	if len(cells) == 0 {
		return nil
	}
	groups := buildJapaneseTokenGroups(seg, cells)
	if len(groups) == 0 {
		groups = make([]japaneseTokenGroup, 0, len(cells))
		for _, cell := range cells {
			groups = append(groups, japaneseTokenGroup{Cells: []japaneseCharCell{cell}})
		}
	}

	pages := make([][]japaneseCharCell, 0, 2)
	for start := 0; start < len(groups); {
		end := chooseJapanesePageBreak(groups, start, layout)
		if end <= start {
			end = start + 1
		}
		pages = append(pages, layoutJapanesePage(flattenJapaneseTokenGroups(groups[start:end]), layout))
		start = end
	}
	return pages
}

func chooseJapanesePageBreak(groups []japaneseTokenGroup, start int, layout subtitleLayout) int {
	if start >= len(groups) {
		return start
	}
	charCount := 0
	width := 0.0
	limit := float64(layout.MaxTextWidth)
	bestEnd := start
	bestPunctEnd := -1
	bestPunctChars := 0

	for i := start; i < len(groups); i++ {
		groupChars := japaneseTokenGroupRuneCount(groups[i])
		if groupChars <= 0 {
			groupChars = 1
		}
		groupWidth := japaneseTokenGroupWidth(groups[i].Cells)
		nextWidth := width
		if i > start && len(groups[i-1].Cells) > 0 {
			prevCells := groups[i-1].Cells
			nextWidth += prevCells[len(prevCells)-1].Gap
		}
		nextWidth += groupWidth
		if i > start && (charCount+groupChars > subtitlePageMaxChars || nextWidth > limit) {
			break
		}

		charCount += groupChars
		width = nextWidth
		bestEnd = i + 1
		if japaneseTokenGroupEndsWithPunctuation(groups[i]) {
			bestPunctEnd = i + 1
			bestPunctChars = charCount
		}
		if charCount >= subtitlePageMaxChars {
			break
		}
	}
	if bestEnd == start {
		return start + 1
	}
	if bestPunctEnd > start && charCount-bestPunctChars <= 4 {
		return bestPunctEnd
	}
	return bestEnd
}

func layoutJapanesePage(cells []japaneseCharCell, layout subtitleLayout) []japaneseCharCell {
	if len(cells) == 0 {
		return nil
	}
	tokenCells := make([]tokenCell, 0, len(cells))
	for _, cell := range cells {
		tokenCells = append(tokenCells, tokenCell{
			Hanzi: cell.Char,
			Width: cell.Width,
			Gap:   cell.Gap,
		})
	}
	centers := computeCellCenters(tokenCells, layout)
	out := make([]japaneseCharCell, 0, len(cells))
	for i, cell := range cells {
		cell.CenterX = centers[i]
		cell.Line = 0
		out = append(out, cell)
	}
	return out
}

func japanesePageStartTimes(pages [][]japaneseCharCell) []int {
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

func japaneseTokenGroupRuneCount(group japaneseTokenGroup) int {
	total := 0
	for _, cell := range group.Cells {
		total += subtitleRuneCount(cell.Char)
	}
	return total
}

func japaneseTokenGroupEndsWithPunctuation(group japaneseTokenGroup) bool {
	if len(group.Cells) == 0 {
		return false
	}
	return subtitleEndsWithPunctuation(group.Cells[len(group.Cells)-1].Char)
}

func buildJapaneseTokenGroups(seg dto.PodcastSegment, cells []japaneseCharCell) []japaneseTokenGroup {
	if len(cells) == 0 {
		return nil
	}
	tokenSpans := seg.TokenSpans
	if len(tokenSpans) == 0 && len(seg.Tokens) > 0 {
		tokenSpans = buildJapaneseTokenSpans(seg)
	}
	if len(tokenSpans) == 0 {
		out := make([]japaneseTokenGroup, 0, len(cells))
		for _, cell := range cells {
			out = append(out, japaneseTokenGroup{Cells: []japaneseCharCell{cell}})
		}
		return out
	}

	out := make([]japaneseTokenGroup, 0, len(cells))
	spanIndex := 0
	for i := 0; i < len(cells); {
		if spanIndex < len(tokenSpans) && cells[i].StartIndex == tokenSpans[spanIndex].StartIndex {
			endCell := i
			for endCell < len(cells) && cells[endCell].EndIndex < tokenSpans[spanIndex].EndIndex {
				endCell++
			}
			if endCell >= len(cells) {
				endCell = len(cells) - 1
			}
			if endCell >= i {
				groupCells := append([]japaneseCharCell(nil), cells[i:endCell+1]...)
				out = append(out, japaneseTokenGroup{Cells: groupCells})
				i = endCell + 1
				spanIndex++
				continue
			}
			spanIndex++
		}
		out = append(out, japaneseTokenGroup{Cells: []japaneseCharCell{cells[i]}})
		i++
	}
	return out
}

func japaneseTokenGroupWidth(cells []japaneseCharCell) float64 {
	return japaneseLineWidth(cells)
}

func flattenJapaneseTokenGroups(groups []japaneseTokenGroup) []japaneseCharCell {
	total := 0
	for _, group := range groups {
		total += len(group.Cells)
	}
	out := make([]japaneseCharCell, 0, total)
	for _, group := range groups {
		out = append(out, group.Cells...)
	}
	return out
}

func japaneseLineWidth(cells []japaneseCharCell) float64 {
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

func japaneseSegmentTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	if len(seg.Tokens) > 0 {
		return seg.Tokens
	}
	text := strings.TrimSpace(seg.Text)
	runes := []rune(text)
	out := make([]dto.PodcastToken, 0, len(runes))
	for _, r := range runes {
		out = append(out, dto.PodcastToken{
			Text: string(r),
		})
	}
	return out
}

func estimateJapaneseCellWidth(text string, layout subtitleLayout) float64 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return estimateTextWidth(text, float64(layout.HanziSize), false)
	}
	w := estimateTextWidth(trimmed, float64(layout.HanziSize), true)
	if isPunctuationText(trimmed) {
		w = maxFloat(w*0.60, float64(layout.HanziSize)*0.48)
	}
	return w
}

func japaneseCharGap(char string, layout subtitleLayout) float64 {
	if isPunctuationText(char) {
		return float64(layout.BaseGap) * 0.30
	}
	return float64(layout.BaseGap) * 0.65
}

func computeJapaneseCellCenters(cells []japaneseCharCell, layout subtitleLayout) []int {
	tokenCells := make([]tokenCell, 0, len(cells))
	for _, cell := range cells {
		tokenCells = append(tokenCells, tokenCell{
			Hanzi: cell.Char,
			Width: cell.Width,
			Gap:   cell.Gap,
		})
	}
	return computeCellCenters(tokenCells, layout)
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

func japaneseRubyCenter(span dto.PodcastTokenSpan, cells []japaneseCharCell) (int, int, bool) {
	var left, right float64
	found := false
	line := 0
	for _, cell := range cells {
		if cell.EndIndex < span.StartIndex || cell.StartIndex > span.EndIndex {
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
	b.WriteString("Style: Ruby," + layout.RubyFontName + "," + strconv.Itoa(layout.RubySize) + "," + layout.RubyColor + "," + layout.RubyColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.RubyBold) + ",0,0,0,100,100,0,0,1,0,0,5,10,10,10,1\n")
	b.WriteString("Style: JaBase," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HanziColor + "," + layout.HanziColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.HanziBold) + ",0,0,0,100,100,0,0,1,0,0,5,10,10,10,1\n")
	b.WriteString("Style: JaActive," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + ",&H00CC66CC,&H00CC66CC," + layout.OutlineColor + ",&H64000000,0,0,0,0,100,100,0,0,1,0,0,5,10,10,10,1\n")
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
			RubyFontName:          "Maruko Gothic CJKjp Light",  // 假名字体
			HanziFontName:         "Maruko Gothic CJKjp Medium", // 日文正文汉字/假名字体
			EnglishFontName:       "Radley",                     // 英文字幕字体
			BoxLeftRatio:          0,                            // 字幕底框左边距占画面宽度比例
			BoxTopRatio:           0.5561,                       // 字幕底框顶部位置占画面高度比例
			BoxWidthRatio:         1,                            // 字幕底框宽度占画面宽度比例
			BoxHeightRatio:        0.4029,                       // 字幕底框高度占画面高度比例
			TextWidthRatio:        0.94,                         // 正文可用排版宽度占底框宽度比例
			TopSectionRatio:       0.7301,                       // 汉字区域高度占底框高度比例
			BottomSectionRatio:    0.2699,                       // 英文区域高度占底框高度比例
			TopSectionTopInset:    0,                            // 上半区顶部额外内边距比例
			BottomSectionTopInset: 0,                            // 下半区顶部额外内边距比例
			RubySize:              39,                           // 假名字号
			HanziSize:             76,                           // 日文正文字号
			EnglishSize:           46,                           // 英文字号
			BaseGap:               1,                            // 字与字之间的基础间距
			RowGap:                1,                            // 假名与正文之间的垂直间距
			TokenLineGap:          16,                           // 两行日文之间的垂直间距
			EnglishLineGap:        8,                            // 英文多行时的行间距
			BoxColor:              "&HFF000000",                 // 底框颜色
			RubyColor:             "&H00000000",                 // 假名颜色
			HanziColor:            "&H00000000",                 // 日文正文颜色
			EnglishColor:          "&H00494393",                 // 英文字幕颜色
			OutlineColor:          "&H00DDD6CF",                 // 轮廓/描边颜色
			RubyBold:              0,                            // 假名是否粗体
			HanziBold:             0,                            // 日文正文是否粗体
			EnglishBold:           1,                            // 英文字幕是否粗体
		}
	case 1:
		fallthrough
	default:
		return subtitlePreset{
			RubyFontName:          "Maruko Gothic CJKjp Light",  // 假名字体
			HanziFontName:         "Maruko Gothic CJKjp Medium", // 日文正文汉字/假名字体
			EnglishFontName:       "Radley",                     // 英文字幕字体
			BoxLeftRatio:          0,                            // 字幕底框左边距占画面宽度比例
			BoxTopRatio:           0.5561,                       // 字幕底框顶部位置占画面高度比例
			BoxWidthRatio:         1,                            // 字幕底框宽度占画面宽度比例
			BoxHeightRatio:        0.4029,                       // 字幕底框高度占画面高度比例
			TextWidthRatio:        0.94,                         // 正文可用排版宽度占底框宽度比例
			TopSectionRatio:       0.7301,                       // 汉字区域高度占底框高度比例
			BottomSectionRatio:    0.2699,                       // 英文区域高度占底框高度比例
			TopSectionTopInset:    0,                            // 上半区顶部额外内边距比例
			BottomSectionTopInset: 0,                            // 下半区顶部额外内边距比例
			RubySize:              39,                           // 假名字号
			HanziSize:             76,                           // 日文正文字号
			EnglishSize:           46,                           // 英文字号
			BaseGap:               1,                            // 字与字之间的基础间距
			RowGap:                1,                            // 假名与正文之间的垂直间距
			TokenLineGap:          16,                           // 两行日文之间的垂直间距
			EnglishLineGap:        6,                            // 英文多行时的行间距
			BoxColor:              "&HFF000000",                 // 底框颜色
			RubyColor:             "&H00000000",                 // 假名颜色
			HanziColor:            "&H00000000",                 // 日文正文颜色
			EnglishColor:          "&H00066F8A",                 // 英文字幕颜色
			OutlineColor:          "&H003B5B55",                 // 轮廓/描边颜色
			RubyBold:              0,                            // 假名是否粗体
			HanziBold:             0,                            // 日文正文是否粗体
			EnglishBold:           1,                            // 英文字幕是否粗体
		}
	}
}

func buildJapaneseTokenSpans(seg dto.PodcastSegment) []dto.PodcastTokenSpan {
	text := strings.TrimSpace(seg.Text)
	if text == "" || len(seg.Tokens) == 0 {
		return nil
	}
	runes := []rune(text)
	out := make([]dto.PodcastTokenSpan, 0, len(seg.Tokens))
	searchFrom := 0
	for _, token := range seg.Tokens {
		surface := strings.TrimSpace(token.Text)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		start, end, ok := findJapaneseRubySurfaceRange(runes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeJapaneseRubySpanRange(runes, dto.PodcastTokenSpan{
			StartIndex: start,
			EndIndex:   end,
			Reading:    reading,
		})
		if !ok {
			searchFrom = end + 1
			continue
		}
		out = append(out, span)
		searchFrom = end + 1
	}
	return dedupeJapaneseTokenSpans(out)
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

func normalizeJapaneseRubySpanRange(runes []rune, span dto.PodcastTokenSpan) (dto.PodcastTokenSpan, bool) {
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
		return dto.PodcastTokenSpan{}, false
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

func dedupeJapaneseTokenSpans(spans []dto.PodcastTokenSpan) []dto.PodcastTokenSpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]dto.PodcastTokenSpan, 0, len(spans))
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
