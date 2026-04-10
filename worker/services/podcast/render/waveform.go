package podcast

import "fmt"

const designType1WaveTopRatio = 370.0 / 775.0

type waveformPreset struct {
	AudioGraph       string
	BackgroundFilter string
	Overlay          string
}

func waveformPresetFor(resolution string, style int, audioInputIndex int) waveformPreset {
	return presetSilkThreadPulse(resolution, style, audioInputIndex)
}

// presetSilkThreadPulse renders a cleaner centered waveform without the extra horizontal guide line.
func presetSilkThreadPulse(resolution string, style int, audioInputIndex int) waveformPreset {
	w, h := resolutionSize(resolution)
	waveW := waveMaxInt(160, int(float64(w)*0.30))
	waveH := waveMaxInt(24, int(float64(h)*0.048))
	y := waveformTopY(h, waveH, style)
	color := waveformColor(style)
	audioGraph := fmt.Sprintf(
		"[%d:a]aformat=channel_layouts=mono,volume=2.40,showwaves=s=%dx%d:mode=cline:rate=30:colors=%s,format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='alpha(X,Y)*(0.28+0.72*pow(max(0,1-abs(2*X/W-1)),1.80))',gblur=sigma=0.30[sw]",
		audioInputIndex,
		waveW, waveH,
		color,
	)
	return waveformPreset{
		AudioGraph:       audioGraph,
		BackgroundFilter: "",
		Overlay:          fmt.Sprintf("(W-w)/2:%d", y),
	}
}

func waveformTopY(height, waveHeight, style int) int {
	if style == 1 {
		return int(float64(height) * designType1WaveTopRatio)
	}
	lineY := int(float64(height) * 0.07)
	return lineY - waveHeight/2
}

func waveformColor(style int) string {
	if style == 1 {
		return "0xFFFFFF"
	}
	return "0x8F4BB3"
}

func waveMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
