package tts

import (
	"fmt"
	"strings"
	"worker/pkg/tts/elevenlabs"
	"worker/pkg/tts/tencent"
)

func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "elevenlabs":
		return elevenlabs.New(cfg)
	case "", "tencent":
		return tencent.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported tts provider: %s", cfg.Provider)
	}
}
