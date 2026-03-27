package podcast

import "fmt"

type waveformPreset struct {
	AudioGraph       string
	BackgroundFilter string
	Overlay          string
}

func waveformPresetFor(resolution string, audioInputIndex int) waveformPreset {
	return presetSilkThreadPulse(resolution, audioInputIndex)
}

// presetSilkThreadPulse renders a cleaner centered waveform without the extra horizontal guide line.
func presetSilkThreadPulse(resolution string, audioInputIndex int) waveformPreset {
	w, h := resolutionSize(resolution)
	waveW := waveMaxInt(160, int(float64(w)*0.23))
	waveH := waveMaxInt(24, int(float64(h)*0.04))
	lineY := int(float64(h) * 0.07)
	y := lineY - waveH/2
	audioGraph := fmt.Sprintf(
		"[%d:a]aformat=channel_layouts=mono,volume=1.85,showwaves=s=%dx%d:mode=cline:rate=30:colors=0x8F4BB3,format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='alpha(X,Y)*(0.16+0.84*pow(max(0,1-abs(2*X/W-1)),2.6))',gblur=sigma=0.45[sw]",
		audioInputIndex,
		waveW, waveH,
	)
	return waveformPreset{
		AudioGraph:       audioGraph,
		BackgroundFilter: "",
		Overlay:          fmt.Sprintf("(W-w)/2:%d", y),
	}
}

func waveMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
