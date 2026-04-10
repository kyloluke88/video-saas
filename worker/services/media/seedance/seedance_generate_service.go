package seedance_generate_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	conf "worker/pkg/config"
	"worker/pkg/seedance"
	"worker/pkg/x/jsonx"
	services "worker/services"
)

type SeedanceGenerateInput struct {
	Prompt      string
	AspectRatio string
	Resolution  string
	DurationSec int
}

func RunSeedanceGenerate(ctx context.Context, input SeedanceGenerateInput, traceDir string, tracePrefix string) (string, error) {
	req := seedance.GenerateRequest{
		Prompt:        strings.TrimSpace(input.Prompt),
		AspectRatio:   defaultString(strings.TrimSpace(input.AspectRatio), "16:9"),
		Resolution:    defaultString(strings.TrimSpace(input.Resolution), "720p"),
		Duration:      normalizeDuration(input.DurationSec),
		GenerateAudio: true,
		FixedLens:     false,
	}

	reqRaw, _ := json.Marshal(req)
	_ = jsonx.WriteRawJSON(traceDir, fmt.Sprintf("seedance_request_body_%s.json", tracePrefix), reqRaw)

	if !conf.Get[bool]("worker.seedance_enabled") {
		log.Printf("⏭️ Seedance API skipped by SEEDANCE_ENABLED=false trace=%s", tracePrefix)
		return "", nil
	}

	result, err := seedance.Generate(ctx, toSeedanceConfig(), req)
	_ = jsonx.WriteRawJSON(traceDir, fmt.Sprintf("seedance_response_body_%s.json", tracePrefix), result.CreateResponseBody)
	if len(result.StatusResponseBody) > 0 {
		_ = jsonx.WriteRawJSON(traceDir, fmt.Sprintf("seedance_status_response_body_%s.json", tracePrefix), result.StatusResponseBody)
	}
	if err != nil {
		var nonRetryable seedance.NonRetryableError
		if errors.As(err, &nonRetryable) {
			return "", services.NonRetryableError{Err: nonRetryable.Err}
		}
		return "", err
	}
	return result.VideoURL, nil
}

func toSeedanceConfig() seedance.Config {
	return seedance.Config{
		BaseURL:         conf.Get[string]("worker.seedance_base_url"),
		GeneratePath:    conf.Get[string]("worker.seedance_generate_path"),
		StatusPath:      conf.Get[string]("worker.seedance_status_path"),
		APIKey:          conf.Get[string]("worker.seedance_api_key"),
		PollIntervalSec: conf.Get[int]("worker.seedance_poll_interval_sec"),
		MaxPollAttempts: conf.Get[int]("worker.seedance_max_poll_attempts"),
		HTTPTimeoutSec:  conf.Get[int]("worker.seedance_http_timeout_sec"),
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func normalizeDuration(sec int) string {
	switch {
	case sec <= 4:
		return "4"
	case sec <= 8:
		return "8"
	default:
		return "12"
	}
}
