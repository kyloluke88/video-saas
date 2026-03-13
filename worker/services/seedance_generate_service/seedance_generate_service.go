package seedance_generate_service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	conf "worker/pkg/config"
	"worker/pkg/helpers"
	"worker/pkg/seedance"
	services "worker/services"
)

type SeedanceGenerateInput struct {
	Prompt      string
	AspectRatio string
	Resolution  string
	DurationSec int
}

func RunSeedanceGenerate(input SeedanceGenerateInput, traceDir string, tracePrefix string) (string, error) {
	req := seedance.GenerateRequest{
		Prompt:        strings.TrimSpace(input.Prompt),
		AspectRatio:   helpers.DefaultString(strings.TrimSpace(input.AspectRatio), "16:9"),
		Resolution:    helpers.DefaultString(strings.TrimSpace(input.Resolution), "720p"),
		Duration:      helpers.NormalizeDuration(input.DurationSec),
		GenerateAudio: true,
		FixedLens:     false,
	}

	reqRaw, _ := json.Marshal(req)
	_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_request_body_%s.json", tracePrefix), reqRaw)

	if !conf.Get[bool]("worker.seedance_enabled") {
		log.Printf("⏭️ Seedance API skipped by SEEDANCE_ENABLED=false trace=%s", tracePrefix)
		return "", nil
	}

	result, err := seedance.Generate(toSeedanceConfig(), req)
	_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_response_body_%s.json", tracePrefix), result.CreateResponseBody)
	if len(result.StatusResponseBody) > 0 {
		_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_status_response_body_%s.json", tracePrefix), result.StatusResponseBody)
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
