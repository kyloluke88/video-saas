package practical_compose_service

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"

	services "worker/services"
	podcastdto "worker/services/podcast/model"
	dto "worker/services/practical/model"
)

type practicalSubtitleStyle struct {
	TurnFontName          string
	RubyFontName          string
	TurnFontSize          int
	BlockFontSize         int
	RubyFontSize          int
	PrimaryColor          string
	OutlineColor          string
	RubyColor             string
	BoxColor              string
	FemaleBoxColor        string
	TurnBold              int
	BlockBold             int
	RubyBold              int
	Outline               int
	TurnMaxLineChars      int
	BlockMaxLineChars     int
	WrapMaxLines          int
	RowGap                int
	RubyGap               int
	TurnBoxPaddingX       int
	TurnBoxPaddingY       int
	BlockBoxPaddingX      int
	BlockBoxPaddingY      int
	TurnBoxRadius         int
	BlockBoxRadius        int
	MinBoxWidthRatio      float64
	MaxBoxWidthRatio      float64
	BlockMinBoxWidthRatio float64
	BlockMaxBoxWidthRatio float64
	TurnPanelTopRatio     float64
	BottomMargin          int
}

func writePracticalASS(script dto.PracticalScript, projectDir, resolution string, designType int) (string, error) {
	turns := flattenTurns(script)
	if len(turns) == 0 {
		return "", nil
	}

	playW, playH := practicalResolutionDimensions(resolution)
	style := practicalSubtitleStyleFor(script.Language, designType)

	var b strings.Builder
	writePracticalASSHeader(&b, playW, playH, style)
	for _, block := range script.Blocks {
		if err := appendPracticalBlockTitleLines(&b, playW, playH, style, block); err != nil {
			return "", err
		}
	}
	for _, turn := range turns {
		if err := appendPracticalTurnLines(&b, playW, playH, style, script.Language, turn); err != nil {
			return "", err
		}
	}
	if b.Len() == 0 {
		return "", nil
	}

	out := projectSubtitleASSPath(projectDir)
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

type practicalTurnEntry struct {
	Turn         dto.PracticalTurn
	SpeakerVoice string
}

type practicalSubtitleLine struct {
	Text      string
	StartRune int
	EndRune   int
}

type practicalSubtitleRow struct {
	RubyTopY int
	BaseTopY int
}

type practicalSubtitlePanel struct {
	Left    int
	Top     int
	Width   int
	Height  int
	CenterX int
	Rows    []practicalSubtitleRow
}

type practicalSubtitleToken struct {
	Surface string
	Reading string
}

type practicalTokenSpan struct {
	StartRune int
	EndRune   int
	Surface   string
	Reading   string
}

type practicalJapaneseRuneGroup struct {
	StartRune int
	EndRune   int
}

type practicalJapaneseLineLayout struct {
	Line  practicalSubtitleLine
	Spans []practicalTokenSpan
	Cells []practicalCharCell
	Width float64
}

type practicalCharCell struct {
	StartRune int
	EndRune   int
	Char      string
	Width     float64
	Gap       float64
	CenterX   int
	Line      int
}

func flattenTurns(script dto.PracticalScript) []practicalTurnEntry {
	out := make([]practicalTurnEntry, 0)
	for _, block := range script.Blocks {
		for _, chapter := range block.Chapters {
			for _, turn := range chapter.Turns {
				out = append(out, practicalTurnEntry{
					Turn:         turn,
					SpeakerVoice: practicalTurnVoice(block, turn),
				})
			}
		}
	}
	return out
}

func appendPracticalBlockTitleLines(b *strings.Builder, playW, playH int, style practicalSubtitleStyle, block dto.PracticalBlock) error {
	startMS, endMS := practicalBlockTitleWindow(block)
	if endMS <= startMS {
		return nil
	}
	text := strings.TrimSpace(block.Topic)
	if text == "" {
		return nil
	}
	lines := buildPracticalSubtitleLines(text, style.BlockMaxLineChars, 2)
	if len(lines) == 0 {
		return nil
	}
	panel := buildPracticalCenteredPanel(playW, playH, style, lines)
	start := formatASSTimestampMS(startMS)
	end := formatASSTimestampMS(endMS)
	b.WriteString(dialogueLineASSLayer(0, "BlockBox", start, end, roundedBoxTextASS(panel.Left, panel.Top, panel.Width, panel.Height, style.BlockBoxRadius)))
	for rowIndex, line := range lines {
		if rowIndex >= len(panel.Rows) {
			break
		}
		b.WriteString(dialogueLineASSLayer(2, "BlockSub", start, end, posTextASSCenter(panel.CenterX, panel.Rows[rowIndex].BaseTopY+practicalBlockLineHeight(style)/2, line.Text)))
	}
	return nil
}

func appendPracticalTurnLines(b *strings.Builder, playW, playH int, style practicalSubtitleStyle, language string, entry practicalTurnEntry) error {
	turn := entry.Turn
	if turn.EndMS <= turn.StartMS {
		return nil
	}
	text := strings.TrimSpace(turn.Text)
	if text == "" {
		return services.NonRetryableError{Err: fmt.Errorf("turn %s text is required for subtitles", strings.TrimSpace(turn.TurnID))}
	}

	maxTextWidth := practicalTurnMaxTextWidth(playW, style)
	lineLayouts := buildPracticalJapaneseLineLayouts(language, text, turn.Tokens, style.TurnFontSize, maxTextWidth, style.TurnMaxLineChars, style.WrapMaxLines)
	lines := make([]practicalSubtitleLine, 0, len(lineLayouts))
	lineWidths := make([]float64, 0, len(lineLayouts))
	if len(lineLayouts) > 0 {
		for _, layout := range lineLayouts {
			lines = append(lines, layout.Line)
			lineWidths = append(lineWidths, layout.Width)
		}
	} else {
		lines = buildPracticalSubtitleLines(text, style.TurnMaxLineChars, style.WrapMaxLines)
		for _, line := range lines {
			lineWidths = append(lineWidths, estimatePracticalTextWidth(line.Text, style.TurnFontSize))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	panel := buildPracticalTurnPanelWithLineWidths(playW, playH, style, lineWidths)
	start := formatASSTimestampMS(maxInt(0, turn.StartMS-practicalSubtitleLeadMS()))
	end := formatASSTimestampMS(turn.EndMS)
	b.WriteString(dialogueLineASSLayer(0, practicalTurnBoxStyleName(entry.SpeakerVoice), start, end, roundedBoxTextASS(panel.Left, panel.Top, panel.Width, panel.Height, style.TurnBoxRadius)))

	spans := buildPracticalTokenSpans(text, turn.Tokens)
	for rowIndex, line := range lines {
		if rowIndex >= len(panel.Rows) {
			break
		}
		row := panel.Rows[rowIndex]
		lineSpans := spans
		if rowIndex < len(lineLayouts) {
			lineSpans = lineLayouts[rowIndex].Spans
		}
		baseCenterY := row.BaseTopY + practicalTurnBaseHeight(style)/2
		rubyCenterY := row.RubyTopY + practicalTurnRubyHeight(style)/2

		if rowIndex < len(lineLayouts) && len(lineLayouts[rowIndex].Cells) > 0 {
			renderCells := layoutPracticalLineCells(lineLayouts[rowIndex].Cells, panel.CenterX)
			for _, cell := range renderCells {
				b.WriteString(dialogueLineASSLayer(1, "TurnSub", start, end, posTextASSCenter(cell.CenterX, baseCenterY, cell.Char)))
			}
			for _, span := range lineSpans {
				centerX, ok := practicalRubyCenter(span, renderCells)
				if !ok {
					continue
				}
				b.WriteString(dialogueLineASSLayer(2, "TurnRuby", start, end, posTextASSCenter(centerX, rubyCenterY, span.Reading)))
			}
			continue
		}

		b.WriteString(dialogueLineASSLayer(1, "TurnSub", start, end, posTextASSCenter(panel.CenterX, baseCenterY, line.Text)))
		if len(lineSpans) == 0 {
			continue
		}
		cells := buildPracticalLineCellsForLine(line, style.TurnFontSize, panel.CenterX)
		for _, span := range lineSpans {
			centerX, ok := practicalRubyCenter(span, cells)
			if !ok {
				continue
			}
			b.WriteString(dialogueLineASSLayer(2, "TurnRuby", start, end, posTextASSCenter(centerX, rubyCenterY, span.Reading)))
		}
	}
	return nil
}

func practicalSubtitleStyleFor(language string, designType int) practicalSubtitleStyle {
	lang := strings.ToLower(strings.TrimSpace(language))
	style := practicalSubtitleStyle{
		TurnFontName:          "HYWenRunSongYun J",
		RubyFontName:          "HYWenRunSongYun J",
		TurnFontSize:          43,
		BlockFontSize:         83,
		RubyFontSize:          23,
		PrimaryColor:          assColorRGB(0, 0, 0),
		OutlineColor:          assColorRGB(0, 0, 0),
		RubyColor:             assColorRGB(0, 0, 0),
		BoxColor:              assColorRGB(248, 221, 160),
		FemaleBoxColor:        assColorRGB(244, 214, 150),
		TurnBold:              1,
		BlockBold:             1,
		RubyBold:              0,
		Outline:               0,
		TurnMaxLineChars:      25,
		BlockMaxLineChars:     25,
		WrapMaxLines:          2,
		RowGap:                8,
		RubyGap:               3,
		TurnBoxPaddingX:       12,
		TurnBoxPaddingY:       8,
		BlockBoxPaddingX:      18,
		BlockBoxPaddingY:      12,
		TurnBoxRadius:         18,
		BlockBoxRadius:        22,
		MinBoxWidthRatio:      0.18,
		MaxBoxWidthRatio:      0.88,
		BlockMinBoxWidthRatio: 0.24,
		BlockMaxBoxWidthRatio: 0.92,
		TurnPanelTopRatio:     0.79,
		BottomMargin:          24,
	}
	if lang == "ja" {
		style.TurnFontName = "Maruko Gothic CJKjp Medium"
		style.RubyFontName = style.TurnFontName
		style.TurnBold = 0
		style.BlockBold = 0
		style.TurnMaxLineChars = 25
		style.BlockMaxLineChars = 25
	}
	switch normalizePracticalDesignType(designType) {
	case 2:
		style.BoxColor = assColorRGB(244, 214, 150)
		style.FemaleBoxColor = assColorRGB(239, 206, 141)
	default:
		style.BoxColor = assColorRGB(248, 221, 160)
		style.FemaleBoxColor = assColorRGB(244, 214, 150)
	}
	return style
}

func writePracticalASSHeader(b *strings.Builder, playW, playH int, style practicalSubtitleStyle) {
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	b.WriteString("Collisions: Normal\n")
	b.WriteString("PlayDepth: 0\n")
	b.WriteString("WrapStyle: 2\n")
	b.WriteString("ScaledBorderAndShadow: yes\n")
	b.WriteString("YCbCr Matrix: TV.601\n")
	b.WriteString("PlayResX: " + strconv.Itoa(playW) + "\n")
	b.WriteString("PlayResY: " + strconv.Itoa(playH) + "\n\n")

	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\n")
	b.WriteString("Style: TurnBox," + style.TurnFontName + ",20," + style.BoxColor + "," + style.BoxColor + "," + style.BoxColor + "," + style.BoxColor + ",0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n")
	b.WriteString("Style: TurnBoxFemale," + style.TurnFontName + ",20," + style.FemaleBoxColor + "," + style.FemaleBoxColor + "," + style.FemaleBoxColor + "," + style.FemaleBoxColor + ",0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n")
	b.WriteString("Style: BlockBox," + style.TurnFontName + ",20," + style.BoxColor + "," + style.BoxColor + "," + style.BoxColor + "," + style.BoxColor + ",0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n")
	b.WriteString("Style: TurnRuby," + style.RubyFontName + "," + strconv.Itoa(style.RubyFontSize) + "," + style.RubyColor + "," + style.RubyColor + "," + style.OutlineColor + ",&H00000000," + strconv.Itoa(style.RubyBold) + ",0,0,0,100,100,0,0,1,0,0,8,0,0,0,1\n")
	b.WriteString("Style: TurnSub," + style.TurnFontName + "," + strconv.Itoa(style.TurnFontSize) + "," + style.PrimaryColor + "," + style.PrimaryColor + "," + style.OutlineColor + ",&H00000000," + strconv.Itoa(style.TurnBold) + ",0,0,0,100,100,0,0,1," + strconv.Itoa(style.Outline) + ",0,8,0,0,0,1\n")
	b.WriteString("Style: BlockSub," + style.TurnFontName + "," + strconv.Itoa(style.BlockFontSize) + "," + style.PrimaryColor + "," + style.PrimaryColor + "," + style.OutlineColor + ",&H00000000," + strconv.Itoa(style.BlockBold) + ",0,0,0,100,100,0,0,1," + strconv.Itoa(style.Outline) + ",0,8,0,0,0,1\n\n")

	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n")
}

func buildPracticalTurnPanel(playW, playH int, style practicalSubtitleStyle, lines []practicalSubtitleLine) practicalSubtitlePanel {
	lineWidths := make([]float64, 0, len(lines))
	for _, line := range lines {
		lineWidths = append(lineWidths, estimatePracticalTextWidth(line.Text, style.TurnFontSize))
	}
	return buildPracticalTurnPanelWithLineWidths(playW, playH, style, lineWidths)
}

func buildPracticalTurnPanelWithLineWidths(playW, playH int, style practicalSubtitleStyle, lineWidths []float64) practicalSubtitlePanel {
	maxWidth := 0
	for _, width := range lineWidths {
		if ceil := int(math.Ceil(width)); ceil > maxWidth {
			maxWidth = ceil
		}
	}

	minBoxWidth := int(float64(playW) * style.MinBoxWidthRatio)
	maxBoxWidth := int(float64(playW) * style.MaxBoxWidthRatio)
	boxWidth := maxInt(maxWidth+style.TurnBoxPaddingX*2, minBoxWidth)
	if maxBoxWidth > 0 && boxWidth > maxBoxWidth {
		boxWidth = maxBoxWidth
	}

	rubyHeight := practicalTurnRubyHeight(style)
	baseHeight := practicalTurnBaseHeight(style)
	rowHeight := rubyHeight + style.RubyGap + baseHeight
	lineCount := maxInt(1, len(lineWidths))
	textHeight := rowHeight * lineCount
	if lineCount > 1 {
		textHeight += style.RowGap * (lineCount - 1)
	}
	boxHeight := textHeight + style.TurnBoxPaddingY*2

	centerX := playW / 2
	left := centerX - boxWidth/2
	if left < 0 {
		left = 0
	}
	if left+boxWidth > playW {
		left = playW - boxWidth
	}

	top := int(float64(playH) * style.TurnPanelTopRatio)
	maxTop := playH - style.BottomMargin - boxHeight
	if maxTop < 0 {
		maxTop = 0
	}
	if top > maxTop {
		top = maxTop
	}
	if top < 0 {
		top = 0
	}

	rows := make([]practicalSubtitleRow, 0, lineCount)
	cursorY := top + style.TurnBoxPaddingY
	for index := 0; index < lineCount; index++ {
		rows = append(rows, practicalSubtitleRow{
			RubyTopY: cursorY,
			BaseTopY: cursorY + rubyHeight + style.RubyGap,
		})
		cursorY += rowHeight + style.RowGap
	}

	return practicalSubtitlePanel{
		Left:    left,
		Top:     top,
		Width:   boxWidth,
		Height:  boxHeight,
		CenterX: centerX,
		Rows:    rows,
	}
}

func buildPracticalCenteredPanel(playW, playH int, style practicalSubtitleStyle, lines []practicalSubtitleLine) practicalSubtitlePanel {
	maxWidth := 0
	for _, line := range lines {
		width := int(math.Ceil(estimatePracticalTextWidth(line.Text, style.BlockFontSize)))
		if width > maxWidth {
			maxWidth = width
		}
	}

	minBoxWidth := int(float64(playW) * style.BlockMinBoxWidthRatio)
	maxBoxWidth := int(float64(playW) * style.BlockMaxBoxWidthRatio)
	boxWidth := maxInt(maxWidth+style.BlockBoxPaddingX*2, minBoxWidth)
	if maxBoxWidth > 0 && boxWidth > maxBoxWidth {
		boxWidth = maxBoxWidth
	}

	lineHeight := practicalBlockLineHeight(style)
	textHeight := lineHeight * maxInt(1, len(lines))
	if len(lines) > 1 {
		textHeight += style.RowGap * (len(lines) - 1)
	}
	boxHeight := textHeight + style.BlockBoxPaddingY*2

	centerX := playW / 2
	left := centerX - boxWidth/2
	if left < 0 {
		left = 0
	}
	if left+boxWidth > playW {
		left = playW - boxWidth
	}

	top := playH/2 - boxHeight/2
	if top < 0 {
		top = 0
	}

	rows := make([]practicalSubtitleRow, 0, len(lines))
	cursorY := top + style.BlockBoxPaddingY
	for range lines {
		rows = append(rows, practicalSubtitleRow{BaseTopY: cursorY})
		cursorY += lineHeight + style.RowGap
	}

	return practicalSubtitlePanel{
		Left:    left,
		Top:     top,
		Width:   boxWidth,
		Height:  boxHeight,
		CenterX: centerX,
		Rows:    rows,
	}
}

func practicalBlockTitleWindow(block dto.PracticalBlock) (int, int) {
	startMS := maxInt(0, block.TopicStartMS+practicalBlockTransitionLeadMS()-practicalSubtitleLeadMS())
	endMS := maxInt(startMS, block.TopicEndMS)
	return startMS, endMS
}

func buildPracticalSubtitleLines(text string, maxChars, maxLines int) []practicalSubtitleLine {
	lineTexts := splitSubtitleLines(text, maxChars, maxLines)
	if len(lineTexts) == 0 {
		return nil
	}
	baseRunes := []rune(strings.TrimSpace(text))
	out := make([]practicalSubtitleLine, 0, len(lineTexts))
	cursor := 0
	for _, lineText := range lineTexts {
		lineRunes := []rune(strings.TrimSpace(lineText))
		if len(lineRunes) == 0 {
			continue
		}
		start := findPracticalRuneSlice(baseRunes, lineRunes, cursor)
		if start < 0 {
			start = cursor
		}
		end := minInt(len(baseRunes), start+len(lineRunes))
		out = append(out, practicalSubtitleLine{
			Text:      strings.TrimSpace(lineText),
			StartRune: start,
			EndRune:   end,
		})
		cursor = end
	}
	return out
}

func parsePracticalSubtitleTokens(raw json.RawMessage) []practicalSubtitleToken {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var parsed []struct {
		Char    string `json:"char"`
		Surface string `json:"surface"`
		Text    string `json:"text"`
		Reading string `json:"reading"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	out := make([]practicalSubtitleToken, 0, len(parsed))
	for _, token := range parsed {
		surface := firstPracticalNonEmpty(
			strings.TrimSpace(token.Char),
			strings.TrimSpace(token.Surface),
			strings.TrimSpace(token.Text),
		)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		out = append(out, practicalSubtitleToken{
			Surface: surface,
			Reading: reading,
		})
	}
	return out
}

func buildPracticalTokenSpans(text string, raw json.RawMessage) []practicalTokenSpan {
	if spans := buildPracticalJapaneseTokenSpans(text, raw); len(spans) > 0 {
		return spans
	}

	tokens := parsePracticalSubtitleTokens(raw)
	if len(tokens) == 0 {
		return nil
	}
	baseRunes := []rune(strings.TrimSpace(text))
	if len(baseRunes) == 0 {
		return nil
	}
	out := make([]practicalTokenSpan, 0, len(tokens))
	cursor := 0
	for _, token := range tokens {
		surfaceRunes := []rune(strings.TrimSpace(token.Surface))
		if len(surfaceRunes) == 0 {
			continue
		}
		start := findPracticalRuneSlice(baseRunes, surfaceRunes, cursor)
		if start < 0 {
			start = findPracticalRuneSlice(baseRunes, surfaceRunes, 0)
		}
		if start < 0 {
			continue
		}
		end := start + len(surfaceRunes)
		out = append(out, practicalTokenSpan{
			StartRune: start,
			EndRune:   end,
			Surface:   token.Surface,
			Reading:   token.Reading,
		})
		cursor = end
	}
	return out
}

func buildPracticalJapaneseTokenSpans(text string, raw json.RawMessage) []practicalTokenSpan {
	tokens := parsePracticalSubtitleTokens(raw)
	if len(tokens) == 0 {
		return nil
	}
	trimmedText := strings.TrimSpace(text)
	baseRunes := []rune(trimmedText)
	if len(baseRunes) == 0 {
		return nil
	}

	podcastTokens := make([]podcastdto.PodcastToken, 0, len(tokens))
	for _, token := range tokens {
		podcastTokens = append(podcastTokens, podcastdto.PodcastToken{
			Char:    token.Surface,
			Reading: token.Reading,
		})
	}
	refs := podcastdto.BuildJapaneseTokenSpanRefs(trimmedText, podcastTokens)
	if len(refs) == 0 {
		return nil
	}

	out := make([]practicalTokenSpan, 0, len(refs))
	for _, ref := range refs {
		start := ref.Span.StartIndex
		end := ref.Span.EndIndex + 1
		if start < 0 || end <= start || end > len(baseRunes) {
			continue
		}
		out = append(out, practicalTokenSpan{
			StartRune: start,
			EndRune:   end,
			Surface:   string(baseRunes[start:end]),
			Reading:   ref.Span.Reading,
		})
	}
	return out
}

type practicalJapaneseCellGroup struct {
	Cells []practicalCharCell
}

func buildPracticalJapaneseLineLayouts(language, text string, raw json.RawMessage, fontSize, maxWidth, maxChars, maxLines int) []practicalJapaneseLineLayout {
	if !strings.EqualFold(strings.TrimSpace(language), "ja") {
		return nil
	}
	trimmedText := strings.TrimSpace(text)
	baseRunes := []rune(trimmedText)
	if len(baseRunes) == 0 {
		return nil
	}

	spans := buildPracticalJapaneseTokenSpans(trimmedText, raw)
	if len(spans) == 0 {
		return nil
	}

	cells := buildPracticalJapaneseCells(baseRunes, fontSize)
	if len(cells) == 0 {
		return nil
	}
	groups := buildPracticalJapaneseCellGroups(cells, spans)
	if len(groups) == 0 {
		return nil
	}
	cellLines := splitPracticalJapaneseCellGroups(groups, maxWidth, maxChars, maxLines)
	if len(cellLines) == 0 {
		return nil
	}

	out := make([]practicalJapaneseLineLayout, 0, len(cellLines))
	for lineIndex, lineCells := range cellLines {
		if len(lineCells) == 0 {
			continue
		}
		startRune := lineCells[0].StartRune
		endRune := lineCells[len(lineCells)-1].EndRune
		if startRune < 0 || endRune <= startRune || endRune > len(baseRunes) {
			continue
		}
		copiedCells := append([]practicalCharCell(nil), lineCells...)
		for cellIndex := range copiedCells {
			copiedCells[cellIndex].Line = lineIndex
		}
		lineSpans := make([]practicalTokenSpan, 0, len(spans))
		for _, span := range spans {
			if span.EndRune <= startRune || span.StartRune >= endRune {
				continue
			}
			lineSpans = append(lineSpans, span)
		}
		out = append(out, practicalJapaneseLineLayout{
			Line: practicalSubtitleLine{
				Text:      strings.TrimSpace(string(baseRunes[startRune:endRune])),
				StartRune: startRune,
				EndRune:   endRune,
			},
			Spans: lineSpans,
			Cells: copiedCells,
			Width: practicalLineWidth(copiedCells),
		})
	}
	return out
}

func buildPracticalJapaneseCells(baseRunes []rune, fontSize int) []practicalCharCell {
	if len(baseRunes) == 0 {
		return nil
	}
	cells := make([]practicalCharCell, 0, len(baseRunes))
	for index, r := range baseRunes {
		char := string(r)
		cells = append(cells, practicalCharCell{
			StartRune: index,
			EndRune:   index + 1,
			Char:      char,
			Width:     estimatePracticalTextWidth(char, fontSize),
			Gap:       practicalJapaneseCharGap(char, fontSize),
		})
	}
	return cells
}

func buildPracticalJapaneseCellGroups(cells []practicalCharCell, spans []practicalTokenSpan) []practicalJapaneseCellGroup {
	if len(cells) == 0 {
		return nil
	}
	groups := make([]practicalJapaneseCellGroup, 0, len(cells))
	spanIndex := 0
	for cellIndex := 0; cellIndex < len(cells); {
		if spanIndex < len(spans) && cells[cellIndex].StartRune == spans[spanIndex].StartRune {
			endCell := cellIndex
			for endCell < len(cells) && cells[endCell].EndRune < spans[spanIndex].EndRune {
				endCell++
			}
			if endCell >= len(cells) {
				endCell = len(cells) - 1
			}
			if endCell >= cellIndex {
				groups = append(groups, practicalJapaneseCellGroup{
					Cells: append([]practicalCharCell(nil), cells[cellIndex:endCell+1]...),
				})
				cellIndex = endCell + 1
				spanIndex++
				continue
			}
			spanIndex++
		}
		groups = append(groups, practicalJapaneseCellGroup{
			Cells: []practicalCharCell{cells[cellIndex]},
		})
		cellIndex++
	}
	return groups
}

func splitPracticalJapaneseCellGroups(groups []practicalJapaneseCellGroup, maxWidth, maxChars, maxLines int) [][]practicalCharCell {
	if len(groups) == 0 {
		return nil
	}
	if maxWidth <= 0 {
		maxWidth = 960
	}
	if maxChars <= 0 {
		maxChars = 25
	}
	if maxLines <= 0 {
		maxLines = 2
	}

	totalRunes := 0
	totalWidth := 0.0
	for _, group := range groups {
		totalRunes += practicalJapaneseGroupRuneCount(group)
		totalWidth += practicalJapaneseGroupWidth(group)
	}
	if totalRunes <= maxChars && totalWidth <= float64(maxWidth) || maxLines == 1 {
		return [][]practicalCharCell{flattenPracticalJapaneseCellGroups(groups)}
	}

	if maxLines == 2 {
		bestIndex := -1
		bestScore := math.MaxFloat64
		leftRunes := 0
		leftWidth := 0.0
		for idx := 0; idx < len(groups)-1; idx++ {
			leftRunes += practicalJapaneseGroupRuneCount(groups[idx])
			leftWidth += practicalJapaneseGroupWidth(groups[idx])
			rightRunes := totalRunes - leftRunes
			rightWidth := totalWidth - leftWidth
			if leftRunes > maxChars || rightRunes > maxChars || leftWidth > float64(maxWidth) || rightWidth > float64(maxWidth) {
				continue
			}
			score := math.Abs(leftWidth - rightWidth)
			if bestIndex == -1 || score < bestScore {
				bestIndex = idx + 1
				bestScore = score
			}
		}
		if bestIndex > 0 {
			return [][]practicalCharCell{
				flattenPracticalJapaneseCellGroups(groups[:bestIndex]),
				flattenPracticalJapaneseCellGroups(groups[bestIndex:]),
			}
		}
	}

	lines := make([][]practicalCharCell, 0, maxLines)
	current := make([]practicalJapaneseCellGroup, 0, len(groups))
	currentRunes := 0
	currentWidth := 0.0
	for _, group := range groups {
		groupRunes := practicalJapaneseGroupRuneCount(group)
		groupWidth := practicalJapaneseGroupWidth(group)
		nextWidth := currentWidth + groupWidth
		if len(current) > 0 {
			nextWidth += practicalJapaneseGroupGapAfter(current[len(current)-1])
		}
		if len(current) > 0 && (currentRunes+groupRunes > maxChars || nextWidth > float64(maxWidth)) && len(lines) < maxLines-1 {
			lines = append(lines, flattenPracticalJapaneseCellGroups(current))
			current = []practicalJapaneseCellGroup{group}
			currentRunes = groupRunes
			currentWidth = groupWidth
			continue
		}
		current = append(current, group)
		currentRunes += groupRunes
		if len(current) == 1 {
			currentWidth = groupWidth
		} else {
			currentWidth = nextWidth
		}
	}
	if len(current) > 0 {
		lines = append(lines, flattenPracticalJapaneseCellGroups(current))
	}
	if len(lines) <= maxLines {
		return lines
	}

	mergedTail := make([]practicalCharCell, 0)
	for _, extra := range lines[maxLines-1:] {
		mergedTail = append(mergedTail, extra...)
	}
	return append(lines[:maxLines-1], mergedTail)
}

func practicalJapaneseGroupRuneCount(group practicalJapaneseCellGroup) int {
	if len(group.Cells) == 0 {
		return 0
	}
	return maxInt(1, group.Cells[len(group.Cells)-1].EndRune-group.Cells[0].StartRune)
}

func practicalJapaneseGroupWidth(group practicalJapaneseCellGroup) float64 {
	return practicalLineWidth(group.Cells)
}

func practicalJapaneseGroupGapAfter(group practicalJapaneseCellGroup) float64 {
	if len(group.Cells) == 0 {
		return 0
	}
	return group.Cells[len(group.Cells)-1].Gap
}

func flattenPracticalJapaneseCellGroups(groups []practicalJapaneseCellGroup) []practicalCharCell {
	total := 0
	for _, group := range groups {
		total += len(group.Cells)
	}
	out := make([]practicalCharCell, 0, total)
	for _, group := range groups {
		out = append(out, group.Cells...)
	}
	return out
}

func practicalLineWidth(cells []practicalCharCell) float64 {
	total := 0.0
	for index, cell := range cells {
		total += cell.Width
		if index != len(cells)-1 {
			total += cell.Gap
		}
	}
	return total
}

func layoutPracticalLineCells(cells []practicalCharCell, centerX int) []practicalCharCell {
	if len(cells) == 0 {
		return nil
	}
	totalWidth := practicalLineWidth(cells)
	cursor := float64(centerX) - totalWidth/2
	out := make([]practicalCharCell, len(cells))
	for index, cell := range cells {
		cell.CenterX = int(cursor + cell.Width/2)
		out[index] = cell
		cursor += cell.Width
		if index != len(cells)-1 {
			cursor += cell.Gap
		}
	}
	return out
}

func practicalJapaneseCharGap(char string, fontSize int) float64 {
	if strings.TrimSpace(char) == "" {
		return 0
	}
	if isPracticalPunctuationRune([]rune(char)[0]) {
		return float64(fontSize) * 0.16
	}
	return float64(fontSize) * 0.08
}

func buildPracticalLineCellsForLine(line practicalSubtitleLine, fontSize int, centerX int) []practicalCharCell {
	text := line.Text
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	widths := make([]float64, len(runes))
	totalWidth := 0.0
	for idx, r := range runes {
		width := estimatePracticalTextWidth(string(r), fontSize)
		widths[idx] = width
		totalWidth += width
	}
	cursor := float64(centerX) - totalWidth/2
	out := make([]practicalCharCell, 0, len(runes))
	for idx, width := range widths {
		startRune := line.StartRune + idx
		out = append(out, practicalCharCell{
			StartRune: startRune,
			EndRune:   startRune + 1,
			Char:      string(runes[idx]),
			Width:     width,
			CenterX:   int(cursor + width/2),
		})
		cursor += width
	}
	return out
}

func practicalRubyCenter(span practicalTokenSpan, cells []practicalCharCell) (int, bool) {
	if span.EndRune <= span.StartRune || len(cells) == 0 {
		return 0, false
	}
	var left, right float64
	found := false
	for _, cell := range cells {
		if cell.EndRune <= span.StartRune || cell.StartRune >= span.EndRune {
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
		found = true
	}
	if !found {
		return 0, false
	}
	return int((left + right) / 2), true
}

func practicalTurnVoice(block dto.PracticalBlock, turn dto.PracticalTurn) string {
	if voice, err := block.ResolveTurnVoice(turn); err == nil {
		return voice
	}
	voice := strings.ToLower(strings.TrimSpace(turn.SpeakerID))
	switch voice {
	case "male", "female":
		return voice
	default:
		return "male"
	}
}

func practicalTurnBoxStyleName(voice string) string {
	if strings.EqualFold(strings.TrimSpace(voice), "female") {
		return "TurnBoxFemale"
	}
	return "TurnBox"
}

func findPracticalRuneSlice(haystack, needle []rune, from int) int {
	if len(needle) == 0 || len(haystack) == 0 || from >= len(haystack) {
		return -1
	}
	maxStart := len(haystack) - len(needle)
	for start := maxInt(0, from); start <= maxStart; start++ {
		match := true
		for idx := range needle {
			if haystack[start+idx] != needle[idx] {
				match = false
				break
			}
		}
		if match {
			return start
		}
	}
	return -1
}

func splitSubtitleLines(text string, maxChars, maxLines int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 32
	}
	if maxLines <= 0 {
		maxLines = 2
	}

	normalized := normalizeSubtitleSpacing(text)
	if runeCount(normalized) <= maxChars || maxLines == 1 {
		return []string{normalized}
	}

	words := strings.Fields(normalized)
	if len(words) == 0 {
		return splitSubtitleRunes(normalized, maxChars, maxLines)
	}
	if len(words) == 1 && runeCount(normalized) > maxChars {
		return splitSubtitleRunes(normalized, maxChars, maxLines)
	}

	lines := make([]string, 0, maxLines)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if runeCount(candidate) > maxChars && current != "" && len(lines) < maxLines-1 {
			lines = append(lines, current)
			current = word
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) <= maxLines {
		return lines
	}
	tail := strings.Join(lines[maxLines-1:], " ")
	return append(lines[:maxLines-1], tail)
}

func splitSubtitleRunes(text string, maxChars, maxLines int) []string {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) == 0 {
		return nil
	}
	if len(rs) <= maxChars || maxLines == 1 {
		return []string{strings.TrimSpace(string(rs))}
	}
	lines := make([]string, 0, maxLines)
	cursor := 0
	for cursor < len(rs) && len(lines) < maxLines-1 {
		end := minInt(len(rs), cursor+maxChars)
		lines = append(lines, strings.TrimSpace(string(rs[cursor:end])))
		cursor = end
	}
	if cursor < len(rs) {
		lines = append(lines, strings.TrimSpace(string(rs[cursor:])))
	}
	return lines
}

func normalizeSubtitleSpacing(text string) string {
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
		"( ", "(",
		"[ ", "[",
		"{ ", "{",
		" '", "'",
		"' ", "'",
		" \"", "\"",
		"\" ", "\"",
	)
	return replacer.Replace(text)
}

func runeCount(text string) int {
	return len([]rune(strings.TrimSpace(text)))
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

func dialogueLineASSLayer(layer int, style, start, end, text string) string {
	return fmt.Sprintf("Dialogue: %d,%s,%s,%s,,0,0,0,,%s\n", layer, start, end, style, text)
}

func posTextASS(x, y int, text string) string {
	return fmt.Sprintf("{\\an8\\pos(%d,%d)}%s", x, y, escapeASSText(text))
}

func posTextASSCenter(x, y int, text string) string {
	return fmt.Sprintf("{\\an5\\pos(%d,%d)}%s", x, y, escapeASSText(text))
}

func roundedBoxTextASS(left, top, width, height, radius int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if radius < 0 {
		radius = 0
	}
	maxRadius := minInt(width, height) / 2
	if radius > maxRadius {
		radius = maxRadius
	}
	right := left + width
	bottom := top + height
	if radius == 0 {
		return fmt.Sprintf("{\\p1}m %d %d l %d %d l %d %d l %d %d{\\p0}", left, top, right, top, right, bottom, left, bottom)
	}

	r := radius
	return fmt.Sprintf(
		"{\\p1}m %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d{\\p0}",
		left+r, top,
		right-r, top,
		right-r/2, top, right, top+r/2, right, top+r,
		right, bottom-r,
		right, bottom-r/2, right-r/2, bottom, right-r, bottom,
		left+r, bottom,
		left+r/2, bottom, left, bottom-r/2, left, bottom-r,
		left, top+r,
		left, top+r/2, left+r/2, top, left+r, top,
	)
}

func practicalTurnMaxTextWidth(playW int, style practicalSubtitleStyle) int {
	maxBoxWidth := int(float64(playW) * style.MaxBoxWidthRatio)
	maxTextWidth := maxBoxWidth - style.TurnBoxPaddingX*2
	return maxInt(1, maxTextWidth)
}

func practicalTurnRubyHeight(style practicalSubtitleStyle) int {
	return maxInt(style.RubyFontSize, int(float64(style.RubyFontSize)*1.08))
}

func practicalTurnBaseHeight(style practicalSubtitleStyle) int {
	return maxInt(style.TurnFontSize, int(float64(style.TurnFontSize)*1.08))
}

func practicalBlockLineHeight(style practicalSubtitleStyle) int {
	return maxInt(style.BlockFontSize, int(float64(style.BlockFontSize)*1.08))
}

func estimatePracticalTextWidth(text string, fontSize int) float64 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	size := float64(fontSize)
	width := 0.0
	for _, r := range trimmed {
		switch {
		case unicode.IsSpace(r):
			width += size * 0.28
		case isPracticalWideRune(r):
			width += size * 0.96
		case isPracticalPunctuationRune(r):
			width += size * 0.46
		default:
			width += size * 0.62
		}
	}
	return width
}

func isPracticalWideRune(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func isPracticalPunctuationRune(r rune) bool {
	const punctuation = ".,!?;:()[]{}\"'，。！？；：（）【】《》「」『』、・ー〜…"
	return strings.ContainsRune(punctuation, r)
}

func escapeASSText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	s = strings.ReplaceAll(s, "\n", `\N`)
	return s
}

func assColorRGB(r, g, b int) string {
	return fmt.Sprintf("&H00%02X%02X%02X", b, g, r)
}

func firstPracticalNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
