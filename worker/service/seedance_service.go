package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"worker/pkg/helpers"
	"worker/pkg/seedance"
)

func RunSeedanceGenerate(cfg Config, payload map[string]interface{}, traceDir string, tracePrefix string) (string, error) {
	if cfg.SeedanceAPIKey == "" {
		return "", errors.New("SEEDANCE_API_KEY is empty")
	}

	req := seedance.GenerateRequest{
		Prompt:        helpers.GetString(payload, "prompt"),
		AspectRatio:   helpers.DefaultString(helpers.GetString(payload, "aspect_ratio"), "16:9"),
		Resolution:    helpers.DefaultString(helpers.GetString(payload, "resolution"), "720p"),
		Duration:      helpers.DefaultString(helpers.GetString(payload, "duration"), "8"),
		GenerateAudio: helpers.GetBool(payload, "generate_audio", false),
		FixedLens:     helpers.GetBool(payload, "fixed_lens", false),
		ImageURLs:     helpers.GetStringSlice(payload, "image_urls"),
	}
	if req.Prompt == "" {
		return "", errors.New("seedance prompt is required")
	}

	reqRaw, _ := json.Marshal(req)
	_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_request_body_%s.json", tracePrefix), reqRaw)

	result, err := seedance.Generate(toSeedanceConfig(cfg), req)
	_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_response_body_%s.json", tracePrefix), result.CreateResponseBody)
	if len(result.StatusResponseBody) > 0 {
		_ = helpers.WriteRawJSON(traceDir, fmt.Sprintf("seedance_status_response_body_%s.json", tracePrefix), result.StatusResponseBody)
	}
	if err != nil {
		var nonRetryable seedance.NonRetryableError
		if errors.As(err, &nonRetryable) {
			return "", NonRetryableError{Err: nonRetryable.Err}
		}
		return "", err
	}
	return result.VideoURL, nil
}

func toSeedanceConfig(cfg Config) seedance.Config {
	return seedance.Config{
		BaseURL:         cfg.SeedanceBaseURL,
		GeneratePath:    cfg.SeedanceGeneratePath,
		StatusPath:      cfg.SeedanceStatusPath,
		APIKey:          cfg.SeedanceAPIKey,
		PollIntervalSec: cfg.SeedancePollIntervalSec,
		MaxPollAttempts: cfg.SeedanceMaxPollAttempts,
		HTTPTimeoutSec:  cfg.SeedanceHTTPTimeoutSec,
	}
}
