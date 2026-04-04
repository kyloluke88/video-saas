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
	highlightEnabled := podcastHighlightEnabled()

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
		pageStartTimes := japanesePageStartTimes(pages)
		if !highlightEnabled {
			pageStartTimes = nil
		}
		pageWindows := buildSubtitlePageWindows(seg.StartMS, seg.EndMS, pageStartTimes, japanesePageWeights(pages))

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
				if highlightEnabled && cell.EndMS > cell.StartMS {
					activeStart, activeEnd, ok := clampWindow(cell.StartMS, cell.EndMS, window.StartMS, window.EndMS)
					if ok {
						b.WriteString(dialogueLine("JaActive", formatASSTimestampMS(activeStart), formatASSTimestampMS(activeEnd), posText(cell.CenterX, row.BaseY, cell.Char)))
					}
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
		if end, ok := inlineLatinWordTokenRun(tokens, i); ok {
			var word strings.Builder
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
			out = append(out, japaneseCharCell{
				StartIndex: i,
				EndIndex:   end,
				Char:       text,
				Width:      estimateTextWidth(text, float64(layout.HanziSize), false),
				Gap:        gap,
				StartMS:    startMS,
				EndMS:      endMS,
			})
			i = end + 1
			continue
		}
		tk := tokens[i]
		if isWhitespaceOnlyText(tk.Char) {
			out = append(out, japaneseCharCell{
				StartIndex: i,
				EndIndex:   i,
				Char:       tk.Char,
				Width:      estimateWhitespaceWidth(tk.Char, float64(layout.HanziSize)),
				Gap:        0,
				StartMS:    tk.StartMS,
				EndMS:      tk.EndMS,
			})
			i++
			continue
		}
		cellChar := tk.Char
		width := estimateJapaneseCellWidth(cellChar, layout)
		gap := japaneseCharGap(cellChar, layout)
		if isQuotePunctuationText(cellChar) {
			width = maxFloat(estimateTextWidth(strings.TrimSpace(cellChar), float64(layout.HanziSize), false)*0.55, float64(layout.HanziSize)*0.18)
			if japaneseQuoteTouchesInlineLatin(tokens, out, i) {
				gap = 0
			}
		}
		out = append(out, japaneseCharCell{
			StartIndex: i,
			EndIndex:   i,
			Char:       cellChar,
			Width:      width,
			Gap:        gap,
			StartMS:    tk.StartMS,
			EndMS:      tk.EndMS,
		})
		i++
	}
	return out
}

func japaneseQuoteTouchesInlineLatin(tokens []dto.PodcastToken, built []japaneseCharCell, idx int) bool {
	prevInline := len(built) > 0 && isInlineEnglishText(built[len(built)-1].Char)
	if prevInline {
		return true
	}
	if idx+1 >= len(tokens) {
		return false
	}
	_, ok := inlineLatinWordTokenRun(tokens, idx+1)
	return ok
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
	charLimit := subtitlePageCharLimit(layout)
	bestEnd := start
	bestPunctEnd := -1
	forcedWrappedBlockBreak := -1

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
		if i > start && (charCount+groupChars > charLimit || nextWidth > limit) {
			break
		}

		charCount += groupChars
		width = nextWidth
		bestEnd = i + 1
		if japaneseTokenGroupEndsWithPunctuation(groups[i]) && !isOpeningWrapperText(japaneseTokenGroupText(groups[i])) {
			bestPunctEnd = i + 1
			if i > start && isBoundarySymbolText(japaneseTokenGroupText(groups[i])) && longWrappedSpanStartsAfterGroups(groups, i, charLimit, limit) {
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
	texts := make([]string, len(groups))
	for i := range groups {
		texts[i] = japaneseTokenGroupText(groups[i])
	}
	adjusted := adjustSubtitlePageBreak(texts, start, candidateEnd)
	if adjusted > candidateEnd {
		units, width := japaneseGroupMetrics(groups, start, adjusted)
		if units > charLimit || width > limit {
			return candidateEnd
		}
	}
	return adjusted
}

func longWrappedSpanStartsAfterGroups(groups []japaneseTokenGroup, breakAt int, charLimit int, widthLimit float64) bool {
	next := nextNonEmptyJapaneseGroupIndex(groups, breakAt+1)
	if next < 0 || !isOpeningWrapperText(japaneseTokenGroupText(groups[next])) {
		return false
	}
	end, ok := findWrappedSpanEndInJapaneseGroups(groups, next)
	if !ok || end <= next {
		return false
	}
	units, width := japaneseGroupMetrics(groups, next, end+1)
	return units > charLimit || width > widthLimit
}

func nextNonEmptyJapaneseGroupIndex(groups []japaneseTokenGroup, start int) int {
	for i := start; i < len(groups); i++ {
		if strings.TrimSpace(japaneseTokenGroupText(groups[i])) != "" {
			return i
		}
	}
	return -1
}

func findWrappedSpanEndInJapaneseGroups(groups []japaneseTokenGroup, openIndex int) (int, bool) {
	if openIndex < 0 || openIndex >= len(groups) {
		return 0, false
	}
	open := strings.TrimSpace(japaneseTokenGroupText(groups[openIndex]))
	if open == "" {
		return 0, false
	}
	close, symmetric, ok := pairedClosingWrapper(open)
	if !ok {
		return 0, false
	}
	depth := 1
	for i := openIndex + 1; i < len(groups); i++ {
		current := strings.TrimSpace(japaneseTokenGroupText(groups[i]))
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

func japaneseGroupMetrics(groups []japaneseTokenGroup, start, end int) (int, float64) {
	if start < 0 {
		start = 0
	}
	if end > len(groups) {
		end = len(groups)
	}
	if end <= start {
		return 0, 0
	}
	units := 0
	width := 0.0
	for i := start; i < end; i++ {
		groupChars := japaneseTokenGroupRuneCount(groups[i])
		if groupChars <= 0 {
			groupChars = 1
		}
		units += groupChars
		if i > start && len(groups[i-1].Cells) > 0 {
			prevCells := groups[i-1].Cells
			width += prevCells[len(prevCells)-1].Gap
		}
		width += japaneseTokenGroupWidth(groups[i].Cells)
	}
	return units, width
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

func japanesePageWeights(pages [][]japaneseCharCell) []int {
	out := make([]int, 0, len(pages))
	for _, page := range pages {
		weight := 0
		for _, cell := range page {
			weight += maxInt(1, subtitleRuneCount(cell.Char))
		}
		out = append(out, maxInt(1, weight))
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

func japaneseTokenGroupText(group japaneseTokenGroup) string {
	if len(group.Cells) == 0 {
		return ""
	}
	var b strings.Builder
	for _, cell := range group.Cells {
		b.WriteString(cell.Char)
	}
	return b.String()
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
	text := strings.TrimSpace(seg.Text)
	runes := []rune(text)
	out := make([]dto.PodcastToken, 0, len(runes))
	for _, r := range runes {
		out = append(out, dto.PodcastToken{
			Char: string(r),
		})
	}
	if len(out) == 0 {
		return nil
	}

	highlightRanges := japaneseDisplayHighlightRanges(seg)
	for _, highlight := range highlightRanges {
		assignJapaneseDisplayRange(&out, highlight.StartIndex, highlight.EndIndex, highlight.StartMS, highlight.EndMS)
	}
	fillJapaneseDisplayTokenTimingGaps(out, seg.StartMS, seg.EndMS)
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
		return layout.HanziSpacing * layout.PunctuationGapRatio
	}
	return layout.HanziSpacing
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
	b.WriteString("Style: Ruby," + layout.RubyFontName + "," + strconv.Itoa(layout.RubySize) + "," + layout.RubyColor + "," + layout.RubyColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.RubyBold) + ",0,0,0,100,100," + assSpacingText(layout.RubySpacing) + ",0,1,0,0,5,10,10,10,1\n")
	b.WriteString("Style: JaBase," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HanziColor + "," + layout.HanziColor + "," + layout.OutlineColor + ",&H64000000," + strconv.Itoa(layout.HanziBold) + ",0,0,0,100,100," + assSpacingText(layout.HanziSpacing) + ",0,1,0,0,5,10,10,10,1\n")
	b.WriteString("Style: JaActive," + layout.HanziFontName + "," + strconv.Itoa(layout.HanziSize) + "," + layout.HighlightColor + "," + layout.HighlightColor + "," + layout.OutlineColor + ",&H64000000,0,0,0,0,100,100," + assSpacingText(layout.HanziSpacing) + ",0,1,0,0,5,10,10,10,1\n")
	b.WriteString("Style: English," + layout.EnglishFontName + "," + strconv.Itoa(layout.EnglishSize) + "," + layout.EnglishColor + "," + layout.EnglishColor + "," + layout.EnglishColor + ",&H00000000," + strconv.Itoa(layout.EnglishBold) + ",0,0,0,100,100," + assSpacingText(layout.EnglishSpacing) + ",0,1,0,0,5,10,10,10,1\n\n")

	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n")
}

func japaneseSubtitleLayout(playW, playH, style int) subtitleLayout {
	return newSubtitleLayout(playW, playH, japaneseSubtitlePresetFor(style))
}

func japaneseSubtitlePresetFor(style int) subtitlePreset {
	switch style {
	case 2:
		return japaneseSubtitlePresetStyle2()
	case 1:
		fallthrough
	default:
		return japaneseSubtitlePresetStyle1()
	}
}

func japaneseSubtitlePresetStyle1() subtitlePreset {
	preset := japaneseSubtitlePresetStyle2()
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
	applyJapaneseDesignType1Typography(&preset)
	return preset
}

func japaneseSubtitlePresetStyle2() subtitlePreset {
	preset := subtitlePreset{
		RubyFontName:          "Maruko Gothic CJKjp Light",  // 假名字体
		HanziFontName:         "Maruko Gothic CJKjp Medium", // 日文正文汉字/假名字体
		EnglishFontName:       "Radley",                     // 英文字幕字体
		BoxLeftRatio:          0,                            // 字幕底框左边距占画面宽度比例
		BoxTopRatio:           0.5561,                       // 字幕底框顶部位置占画面高度比例
		BoxWidthRatio:         1,                            // 字幕底框宽度占画面宽度比例
		BoxHeightRatio:        0.4029,                       // 字幕底框高度占画面高度比例
		TextWidthRatio:        0.94,                         // 正文可用排版宽度占底框宽度比例
		TopSectionRatio:       0.7101,                       // 汉字区域高度占底框高度比例
		BottomSectionRatio:    0.2699,                       // 英文区域高度占底框高度比例
		TopSectionTopInset:    0.02,                         // 上半区顶部额外内边距比例
		TopSectionOffsetRatio: 0.03,
		BottomSectionTopInset: 0,  // 下半区顶部额外内边距比例
		RubySize:              36, // 假名字号
		HanziSize:             76, // 日文正文字号
		EnglishSize:           46, // 英文字号
		BaseGap:               1,  // 字与字之间的基础间距
		HanziSpacing:          0.1083333333,
		RubySpacing:           -2, // ruby 假名字距
		EnglishSpacing:        0,  // 英文字距
		PunctuationGapRatio:   0.4615384615,
		MaxLineChars:          20,                        // 正文每行最大字符数
		RowGap:                1,                         // 假名与正文之间的垂直间距
		TokenLineGap:          16,                        // 两行日文之间的垂直间距
		EnglishLineGap:        8,                         // 英文多行时的行间距
		BoxColor:              "&HFF000000",              // 底框颜色
		RubyColor:             "&H00000000",              // 假名颜色
		HanziColor:            "&H00000000",              // 日文正文颜色
		HighlightColor:        "&H00CC66CC",              // 高亮颜色
		EnglishColor:          assColorRGB(183, 236, 70), // 英文字幕颜色
		OutlineColor:          "&H00DDD6CF",              // 轮廓/描边颜色
		RubyBold:              0,                         // 假名是否粗体
		HanziBold:             0,                         // 日文正文是否粗体
		EnglishBold:           1,                         // 英文字幕是否粗体
	}
	applyJapaneseDesignType1Typography(&preset)
	return preset
}

func applyJapaneseDesignType1Typography(preset *subtitlePreset) {
	preset.RubySize = 38
	preset.HanziSize = 78
	preset.EnglishSize = 49
	preset.RubyBold = 0
	preset.HanziBold = 0
	preset.EnglishBold = 1
}

// Keep a stable "base" entry for future style tuning workflows.
func japaneseSubtitlePresetBase() subtitlePreset {
	return japaneseSubtitlePresetStyle2()
}

func buildJapaneseTokenSpans(seg dto.PodcastSegment) []dto.PodcastTokenSpan {
	if len(seg.TokenSpans) > 0 {
		return seg.TokenSpans
	}
	return dto.BuildJapaneseTokenSpans(strings.TrimSpace(seg.Text), seg.Tokens)
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

type japaneseAnnotationDetail struct {
	Span    dto.PodcastTokenSpan
	StartMS int
	EndMS   int
}

type japaneseHighlightRange struct {
	StartIndex int
	EndIndex   int
	StartMS    int
	EndMS      int
}

func buildJapaneseAnnotationDetails(seg dto.PodcastSegment) []japaneseAnnotationDetail {
	refs := dto.BuildJapaneseTokenSpanRefs(strings.TrimSpace(seg.Text), seg.Tokens)
	if len(refs) == 0 {
		return nil
	}
	out := make([]japaneseAnnotationDetail, 0, len(refs))
	for _, ref := range refs {
		if ref.TokenIndex < 0 || ref.TokenIndex >= len(seg.Tokens) {
			continue
		}
		token := seg.Tokens[ref.TokenIndex]
		if strings.TrimSpace(token.Char) == "" || strings.TrimSpace(token.Reading) == "" {
			continue
		}
		out = append(out, japaneseAnnotationDetail{
			Span:    ref.Span,
			StartMS: token.StartMS,
			EndMS:   token.EndMS,
		})
	}
	return out
}

func buildJapaneseHighlightRanges(seg dto.PodcastSegment) []japaneseHighlightRange {
	details := buildJapaneseAnnotationDetails(seg)
	if len(details) == 0 {
		return nil
	}

	runes := []rune(strings.TrimSpace(seg.Text))
	out := make([]japaneseHighlightRange, 0, len(details))
	current := japaneseHighlightRange{
		StartIndex: details[0].Span.StartIndex,
		EndIndex:   details[0].Span.EndIndex,
		StartMS:    details[0].StartMS,
		EndMS:      details[0].EndMS,
	}

	for i := 1; i < len(details); i++ {
		next := details[i]
		if shouldMergeJapaneseHighlightRange(runes, current, next) {
			current.EndIndex = next.Span.EndIndex
			if next.StartMS < current.StartMS {
				current.StartMS = next.StartMS
			}
			if next.EndMS > current.EndMS {
				current.EndMS = next.EndMS
			}
			continue
		}

		out = append(out, current)
		current = japaneseHighlightRange{
			StartIndex: next.Span.StartIndex,
			EndIndex:   next.Span.EndIndex,
			StartMS:    next.StartMS,
			EndMS:      next.EndMS,
		}
	}

	out = append(out, current)
	return out
}

func japaneseDisplayHighlightRanges(seg dto.PodcastSegment) []japaneseHighlightRange {
	if len(seg.HighlightSpans) > 0 {
		out := make([]japaneseHighlightRange, 0, len(seg.HighlightSpans))
		for _, span := range seg.HighlightSpans {
			if span.EndIndex < span.StartIndex || span.EndMS <= span.StartMS {
				continue
			}
			out = append(out, japaneseHighlightRange{
				StartIndex: span.StartIndex,
				EndIndex:   span.EndIndex,
				StartMS:    span.StartMS,
				EndMS:      span.EndMS,
			})
		}
		if len(out) > 0 {
			return out
		}
	}
	return buildJapaneseHighlightRanges(seg)
}

func shouldMergeJapaneseHighlightRange(runes []rune, current japaneseHighlightRange, next japaneseAnnotationDetail) bool {
	if next.StartMS > current.EndMS {
		return false
	}
	if next.Span.StartIndex <= current.EndIndex {
		return true
	}
	if current.EndIndex < 0 || next.Span.StartIndex > len(runes) || current.EndIndex+1 > next.Span.StartIndex {
		return false
	}
	return japaneseRuneGapLooksLikeSameWord(runes[current.EndIndex+1 : next.Span.StartIndex])
}

func japaneseRuneGapLooksLikeSameWord(gap []rune) bool {
	if len(gap) == 0 {
		return true
	}
	for _, r := range gap {
		switch {
		case unicode.IsSpace(r):
			continue
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			continue
		case r == 'ー' || r == '〜':
			continue
		default:
			return false
		}
	}
	return true
}

func assignJapaneseDisplayRange(tokens *[]dto.PodcastToken, start, end, startMS, endMS int) {
	if tokens == nil || start < 0 || end < start || start >= len(*tokens) {
		return
	}
	if end >= len(*tokens) {
		end = len(*tokens) - 1
	}
	if endMS <= startMS {
		return
	}
	count := end - start + 1
	step := float64(endMS-startMS) / float64(maxInt(count, 1))
	for i := start; i <= end; i++ {
		offset := i - start
		charStart := startMS + int(step*float64(offset))
		charEnd := startMS + int(step*float64(offset+1))
		if i == end {
			charEnd = endMS
		}
		if charEnd <= charStart {
			charEnd = charStart + 1
		}
		(*tokens)[i].StartMS = charStart
		(*tokens)[i].EndMS = charEnd
	}
}

func fillJapaneseDisplayTokenTimingGaps(tokens []dto.PodcastToken, segmentStartMS, segmentEndMS int) {
	if len(tokens) == 0 {
		return
	}
	start := maxInt(0, segmentStartMS)
	end := maxInt(start+1, segmentEndMS)

	for i := 0; i < len(tokens); {
		if tokens[i].EndMS > tokens[i].StartMS {
			i++
			continue
		}

		j := i
		for j < len(tokens) && tokens[j].EndMS <= tokens[j].StartMS {
			j++
		}

		windowStart := start
		if i > 0 && tokens[i-1].EndMS > tokens[i-1].StartMS {
			windowStart = tokens[i-1].EndMS
		}
		windowEnd := end
		if j < len(tokens) && tokens[j].EndMS > tokens[j].StartMS {
			windowEnd = tokens[j].StartMS
		}
		if windowEnd <= windowStart {
			windowEnd = windowStart + maxInt(j-i, 1)
		}

		step := maxInt(1, (windowEnd-windowStart)/maxInt(j-i, 1))
		cursor := windowStart
		for k := i; k < j; k++ {
			tokens[k].StartMS = cursor
			if k == j-1 {
				tokens[k].EndMS = maxInt(cursor+1, windowEnd)
			} else {
				tokens[k].EndMS = maxInt(cursor+1, cursor+step)
			}
			cursor = tokens[k].EndMS
		}
		i = j
	}
}
