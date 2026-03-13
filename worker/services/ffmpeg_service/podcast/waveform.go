package podcast

import "fmt"

type waveformPreset struct {
	AudioGraph       string
	BackgroundFilter string
	Overlay          string
}

func waveformPresetFor(style int, resolution string) waveformPreset {
	_ = style
	return presetSilkThreadPulse(resolution)
}

// presetSilkThreadPulse matches the thin decorative dashed baseline with a restrained center pulse.
func presetSilkThreadPulse(resolution string) waveformPreset {
	w, h := resolutionSize(resolution)
	waveW := waveMaxInt(120, int(float64(w)*0.13))
	waveH := waveMaxInt(24, int(float64(h)*0.05))
	lineT := waveMaxInt(2, h/720)
	lineY := int(float64(h) * 0.07)
	y := lineY - waveH/2
	lineX := (w - waveW) / 2

	centerDashW := waveMaxInt(40, int(float64(waveW)*0.24))
	centerDashX := lineX + (waveW-centerDashW)/2
	bgFilter := joinFilters([]string{
		dashedLineFilter(lineX, lineY, waveW, lineT, 10, 7, "0x3A1A16@0.88"),
		dashedLineFilter(centerDashX, lineY, centerDashW, lineT, 10, 7, "0xA01F2A@0.95"),
	})
	audioGraph := fmt.Sprintf(
		"[1:a]aformat=channel_layouts=mono,volume=2.35,showwaves=s=%dx%d:mode=p2p:rate=30:colors=0x5B221F,format=rgba[sw]",
		waveW, waveH,
	)
	return waveformPreset{
		AudioGraph:       audioGraph,
		BackgroundFilter: bgFilter,
		Overlay:          fmt.Sprintf("(W-w)/2:%d", y),
	}
}

func waveMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dashedLineFilter(x, y, width, thickness, dashWidth, gapWidth int, color string) string {
	if width <= 0 || dashWidth <= 0 {
		return ""
	}
	step := dashWidth + gapWidth
	parts := make([]string, 0, width/step+1)
	for offset := 0; offset < width; offset += step {
		w := dashWidth
		if offset+w > width {
			w = width - offset
		}
		if w <= 0 {
			break
		}
		parts = append(parts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s:t=fill", x+offset, y, w, thickness, color))
	}
	return joinFilters(parts)
}

func joinFilters(filters []string) string {
	out := ""
	for _, f := range filters {
		if f == "" {
			continue
		}
		if out == "" {
			out = f
			continue
		}
		out += "," + f
	}
	return out
}
