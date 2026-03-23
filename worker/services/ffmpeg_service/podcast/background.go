package podcast

import "fmt"

func backgroundGraphFor(style int, resolution string) string {
	w, h := resolutionSize(resolution)
	switch style {
	case 2:
		return softParallaxBackgroundGraph(w, h)
	case 3:
		return studyGlowBackgroundGraph(w, h)
	default:
		return calmDriftBackgroundGraph(w, h)
	}
}

func calmDriftBackgroundGraph(w, h int) string {
	return fmt.Sprintf(
		"[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d:x='(in_w-out_w)/2 + ((in_w-out_w)*0.18)*sin(t/24)':y='(in_h-out_h)/2 + ((in_h-out_h)*0.10)*sin(t/31)',eq=brightness=0.01:saturation=0.98[bg]",
		enlargedCanvas(w, 1.10),
		enlargedCanvas(h, 1.10),
		w,
		h,
	)
}

func softParallaxBackgroundGraph(w, h int) string {
	return fmt.Sprintf(
		"[0:v]split=2[bg_back_src][bg_front_src];"+
			"[bg_back_src]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d:x='(in_w-out_w)/2 - ((in_w-out_w)*0.22)*sin(t/28)':y='(in_h-out_h)/2 - ((in_h-out_h)*0.14)*sin(t/36)',gblur=sigma=22,eq=brightness=-0.015:saturation=0.88[bg_back];"+
			"[bg_front_src]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d:x='(in_w-out_w)/2 + ((in_w-out_w)*0.10)*sin(t/20)':y='(in_h-out_h)/2 + ((in_h-out_h)*0.06)*sin(t/27)',format=rgba,colorchannelmixer=aa=0.88,eq=brightness=0.01:saturation=1.02[bg_front];"+
			"[bg_back][bg_front]overlay=0:0:format=auto[bg]",
		enlargedCanvas(w, 1.18),
		enlargedCanvas(h, 1.18),
		w,
		h,
		enlargedCanvas(w, 1.08),
		enlargedCanvas(h, 1.08),
		w,
		h,
	)
}

func studyGlowBackgroundGraph(w, h int) string {
	return fmt.Sprintf(
		"[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d:x='(in_w-out_w)/2 + ((in_w-out_w)*0.12)*sin(t/26)':y='(in_h-out_h)/2 + ((in_h-out_h)*0.08)*sin(t/34)',eq=brightness='0.012+0.008*sin(t/11)':saturation=0.94[bg_base];"+
			"color=c=0xF6D49A:s=%dx%d:r=30,format=rgba,colorchannelmixer=aa=0.055[bg_glow];"+
			"[bg_base][bg_glow]overlay=0:0:format=auto[bg]",
		enlargedCanvas(w, 1.12),
		enlargedCanvas(h, 1.12),
		w,
		h,
		w,
		h,
	)
}

func enlargedCanvas(base int, factor float64) int {
	return int(float64(base)*factor + 0.5)
}
