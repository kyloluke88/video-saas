package tencent

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"worker/pkg/tts/shared"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	profile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencenttts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tts/v20190823"
)

type Provider struct {
	region          string
	secretID        string
	secretKey       string
	voiceType       int64
	primaryLanguage int64
	modelType       int64
	codec           string
}

func New(cfg shared.Config) (*Provider, error) {
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
	return &Provider{
		region:          region,
		secretID:        cfg.TencentSecretID,
		secretKey:       cfg.TencentSecretKey,
		voiceType:       cfg.TencentVoiceType,
		primaryLanguage: cfg.TencentPrimaryLanguage,
		modelType:       cfg.TencentModelType,
		codec:           codec,
	}, nil
}

func (p *Provider) Synthesize(ctx context.Context, req shared.Request) (shared.Result, error) {
	cred := common.NewCredential(p.secretID, p.secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "tts.tencentcloudapi.com"
	client, err := tencenttts.NewClient(cred, p.region, cpf)
	if err != nil {
		return shared.Result{}, err
	}

	request := tencenttts.NewTextToVoiceRequest()
	request.Text = common.StringPtr(req.Text)
	request.SessionId = common.StringPtr(fmt.Sprintf("sid-%d", time.Now().UnixNano()))
	request.ModelType = common.Int64Ptr(p.modelType)
	voiceType := p.voiceType
	if req.VoiceType != nil && *req.VoiceType != 0 {
		voiceType = *req.VoiceType
	}
	request.VoiceType = common.Int64Ptr(voiceType)
	request.PrimaryLanguage = common.Int64Ptr(p.primaryLanguage)
	request.Codec = common.StringPtr(strings.ToLower(p.codec))
	if req.Speed != nil {
		request.Speed = common.Float64Ptr(*req.Speed)
	}
	if req.SampleRate != nil && *req.SampleRate > 0 {
		request.SampleRate = common.Uint64Ptr(uint64(*req.SampleRate))
	}
	if value := strings.TrimSpace(req.EmotionCategory); value != "" {
		request.EmotionCategory = common.StringPtr(value)
	}
	if req.EmotionIntensity != nil && *req.EmotionIntensity != 0 {
		request.EmotionIntensity = common.Int64Ptr(*req.EmotionIntensity)
	}
	if req.EnableSubtitle != nil {
		request.EnableSubtitle = common.BoolPtr(*req.EnableSubtitle)
	}

	response, err := client.TextToVoice(request)
	if err != nil {
		if sdkErr, ok := err.(*tcerr.TencentCloudSDKError); ok {
			return shared.Result{}, fmt.Errorf("tencent tts sdk error code=%s msg=%s", sdkErr.Code, sdkErr.Message)
		}
		return shared.Result{}, err
	}
	if response == nil || response.Response == nil || response.Response.Audio == nil || *response.Response.Audio == "" {
		return shared.Result{}, nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(*response.Response.Audio)
	if err != nil {
		return shared.Result{}, err
	}
	ext := strings.ToLower(strings.TrimSpace(p.codec))
	if ext == "" {
		ext = "mp3"
	}
	return shared.Result{
		Audio:     audioBytes,
		Ext:       ext,
		Subtitles: convertSubtitles(response.Response.Subtitles),
	}, nil
}

func convertSubtitles(items []*tencenttts.Subtitle) []shared.Subtitle {
	if len(items) == 0 {
		return nil
	}
	out := make([]shared.Subtitle, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, shared.Subtitle{
			Text:       stringPtrValue(item.Text),
			BeginTime:  int(int64PtrValue(item.BeginTime)),
			EndTime:    int(int64PtrValue(item.EndTime)),
			BeginIndex: int(int64PtrValue(item.BeginIndex)),
			EndIndex:   int(int64PtrValue(item.EndIndex)),
			Phoneme:    stringPtrValue(item.Phoneme),
		})
	}
	return out
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64PtrValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
