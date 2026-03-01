package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	profile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tts/v20190823"
)

type Config struct {
	Provider string

	APIURL string
	APIKey string

	TencentRegion          string
	TencentSecretID        string
	TencentSecretKey       string
	TencentVoiceType       int64
	TencentPrimaryLanguage int64
	TencentModelType       int64
	TencentCodec           string
}

type Request struct {
	Text     string
	Language string
}

type Result struct {
	Audio []byte
	Ext   string
}

type Provider interface {
	Synthesize(ctx context.Context, req Request) (Result, error)
}

func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "http":
		if cfg.APIURL == "" || cfg.APIKey == "" {
			return nil, errors.New("tts http provider not configured")
		}
		return &httpProvider{
			apiURL: cfg.APIURL,
			apiKey: cfg.APIKey,
		}, nil
	case "tencent":
		if cfg.TencentSecretID == "" || cfg.TencentSecretKey == "" {
			return nil, errors.New("tencent tts credentials are required")
		}
		region := strings.TrimSpace(cfg.TencentRegion)
		if region == "" {
			region = "ap-guangzhou"
		}
		codec := strings.TrimSpace(strings.ToLower(cfg.TencentCodec))
		if codec == "" {
			codec = "mp3"
		}
		if cfg.TencentVoiceType == 0 {
			cfg.TencentVoiceType = 101001
		}
		if cfg.TencentPrimaryLanguage == 0 {
			cfg.TencentPrimaryLanguage = 1
		}
		if cfg.TencentModelType == 0 {
			cfg.TencentModelType = 1
		}
		return &tencentProvider{
			region:          region,
			secretID:        cfg.TencentSecretID,
			secretKey:       cfg.TencentSecretKey,
			voiceType:       cfg.TencentVoiceType,
			primaryLanguage: cfg.TencentPrimaryLanguage,
			modelType:       cfg.TencentModelType,
			codec:           codec,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tts provider: %s", cfg.Provider)
	}
}

type httpProvider struct {
	apiURL string
	apiKey string
}

func (p *httpProvider) Synthesize(ctx context.Context, req Request) (Result, error) {
	body := map[string]interface{}{
		"text": req.Text,
	}
	if req.Language != "" {
		body["language"] = req.Language
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(raw))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("tts http failed status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		AudioURL string `json:"audio_url"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Result{}, err
	}
	if parsed.AudioURL == "" {
		return Result{}, nil
	}

	audioResp, err := client.Get(parsed.AudioURL)
	if err != nil {
		return Result{}, err
	}
	defer audioResp.Body.Close()
	if audioResp.StatusCode >= 300 {
		body, _ := io.ReadAll(audioResp.Body)
		return Result{}, fmt.Errorf("download tts audio failed status=%d body=%s", audioResp.StatusCode, string(body))
	}
	audioBytes, err := io.ReadAll(audioResp.Body)
	if err != nil {
		return Result{}, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.AudioURL)), ".")
	if ext == "" {
		ext = "mp3"
	}
	return Result{Audio: audioBytes, Ext: ext}, nil
}

type tencentProvider struct {
	region          string
	secretID        string
	secretKey       string
	voiceType       int64
	primaryLanguage int64
	modelType       int64
	codec           string
}

func (p *tencentProvider) Synthesize(ctx context.Context, req Request) (Result, error) {
	cred := common.NewCredential(p.secretID, p.secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "tts.tencentcloudapi.com"
	client, err := tts.NewClient(cred, p.region, cpf)
	if err != nil {
		return Result{}, err
	}

	request := tts.NewTextToVoiceRequest()
	request.Text = common.StringPtr(req.Text)
	request.SessionId = common.StringPtr(fmt.Sprintf("sid-%d", time.Now().UnixNano()))
	request.ModelType = common.Int64Ptr(p.modelType)
	request.VoiceType = common.Int64Ptr(p.voiceType)
	request.PrimaryLanguage = common.Int64Ptr(p.primaryLanguage)
	request.Codec = common.StringPtr(strings.ToLower(p.codec))

	response, err := client.TextToVoice(request)
	if err != nil {
		if sdkErr, ok := err.(*tcerr.TencentCloudSDKError); ok {
			return Result{}, fmt.Errorf("tencent tts sdk error code=%s msg=%s", sdkErr.Code, sdkErr.Message)
		}
		return Result{}, err
	}
	if response == nil || response.Response == nil || response.Response.Audio == nil || *response.Response.Audio == "" {
		return Result{}, nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(*response.Response.Audio)
	if err != nil {
		return Result{}, err
	}
	ext := strings.ToLower(strings.TrimSpace(p.codec))
	if ext == "" {
		ext = "mp3"
	}
	return Result{Audio: audioBytes, Ext: ext}, nil
}
