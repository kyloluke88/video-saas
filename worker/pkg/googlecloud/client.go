package googlecloud

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	services "worker/services"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

var ttsRequestRetryDelays = []time.Duration{
	3 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

type Config struct {
	ProjectID   string
	UserProject string

	AccessToken        string
	ServiceAccountPath string
	ServiceAccountJSON string
	TokenURL           string

	TTSURL string

	TTSModel         string
	TTSAudioEncoding string
	TTSSampleRateHz  int
	TTSSpeakingRate  float64

	MaleVoiceID        string
	FemaleVoiceID      string
	HTTPTimeoutSeconds int
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	tokenSrc   tokenSource
}

type tokenSource interface {
	Token(context.Context) (string, error)
}

type staticTokenSource struct {
	token string
}

func (s staticTokenSource) Token(context.Context) (string, error) {
	if strings.TrimSpace(s.token) == "" {
		return "", errors.New("google access token is empty")
	}
	return s.token, nil
}

type serviceAccountTokenSource struct {
	creds      serviceAccountCredentials
	tokenURL   string
	httpClient *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

type serviceAccountCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type SynthesizeConversationRequest struct {
	LanguageCode  string
	Prompt        string
	Turns         []ConversationTurn
	MaleVoiceID   string
	FemaleVoiceID string
	SpeakingRate  float64
}

type ConversationTurn struct {
	Speaker string
	Text    string
}

type SynthesizeSingleRequest struct {
	LanguageCode string
	Prompt       string
	Text         string
	VoiceID      string
	SpeakingRate float64
}

type AudioResult struct {
	Audio       []byte
	Ext         string
	RawResponse []byte
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.TokenURL) == "" {
		cfg.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if strings.TrimSpace(cfg.TTSURL) == "" {
		cfg.TTSURL = "https://texttospeech.googleapis.com/v1/text:synthesize"
	}
	if strings.TrimSpace(cfg.TTSModel) == "" {
		cfg.TTSModel = "gemini-2.5-pro-tts"
	}
	if strings.TrimSpace(cfg.TTSAudioEncoding) == "" {
		cfg.TTSAudioEncoding = "MP3"
	}
	if cfg.TTSSampleRateHz <= 0 {
		cfg.TTSSampleRateHz = 24000
	}
	if cfg.TTSSpeakingRate <= 0 {
		cfg.TTSSpeakingRate = 1.0
	}
	if cfg.HTTPTimeoutSeconds <= 0 {
		cfg.HTTPTimeoutSeconds = 180
	}

	httpClient := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second}
	tokenSrc, err := newTokenSource(cfg, httpClient)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		tokenSrc:   tokenSrc,
	}, nil
}

// SynthesizeConversation calls Gemini-TTS multi-speaker synthesis using structured turns.
func (c *Client) SynthesizeConversation(ctx context.Context, req SynthesizeConversationRequest) (AudioResult, error) {
	if len(req.Turns) == 0 {
		return AudioResult{}, errors.New("gemini multi-speaker turns are required")
	}
	maleVoice := strings.TrimSpace(req.MaleVoiceID)
	if maleVoice == "" {
		maleVoice = strings.TrimSpace(c.cfg.MaleVoiceID)
	}
	femaleVoice := strings.TrimSpace(req.FemaleVoiceID)
	if femaleVoice == "" {
		femaleVoice = strings.TrimSpace(c.cfg.FemaleVoiceID)
	}
	if maleVoice == "" || femaleVoice == "" {
		return AudioResult{}, errors.New("gemini multi-speaker voice ids are required")
	}

	turns := make([]map[string]string, 0, len(req.Turns))
	for _, turn := range req.Turns {
		text := strings.TrimSpace(turn.Text)
		if text == "" {
			continue
		}
		speaker := normalizeSpeaker(turn.Speaker)
		turns = append(turns, map[string]string{
			"speaker": speaker,
			"text":    text,
		})
	}
	if len(turns) == 0 {
		return AudioResult{}, errors.New("gemini multi-speaker turns are empty")
	}

	body := map[string]any{
		"input": map[string]any{
			"multiSpeakerMarkup": map[string]any{
				"turns": turns,
			},
		},
		"voice": map[string]any{
			"languageCode": normalizeLanguageCode(req.LanguageCode),
			"modelName":    c.cfg.TTSModel,
			"multiSpeakerVoiceConfig": map[string]any{
				"speakerVoiceConfigs": []map[string]string{
					{"speakerAlias": "male", "speakerId": maleVoice},
					{"speakerAlias": "female", "speakerId": femaleVoice},
				},
			},
		},
		"audioConfig": map[string]any{
			"audioEncoding":   strings.ToUpper(strings.TrimSpace(c.cfg.TTSAudioEncoding)),
			"sampleRateHertz": c.cfg.TTSSampleRateHz,
		},
	}
	if speakingRate := firstPositiveFloat(req.SpeakingRate, c.cfg.TTSSpeakingRate); speakingRate > 0 {
		body["audioConfig"].(map[string]any)["speakingRate"] = speakingRate
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["input"].(map[string]any)["prompt"] = prompt
	}

	var resp struct {
		AudioContent string `json:"audioContent"`
	}
	raw, err := c.doJSONWithRetry(ctx, http.MethodPost, c.cfg.TTSURL, body, &resp)
	if err != nil {
		return AudioResult{}, err
	}
	if strings.TrimSpace(resp.AudioContent) == "" {
		return AudioResult{}, errors.New("gemini tts returned empty audio_content")
	}

	audio, err := base64.StdEncoding.DecodeString(resp.AudioContent)
	if err != nil {
		return AudioResult{}, err
	}
	return AudioResult{
		Audio:       audio,
		Ext:         audioExtForEncoding(c.cfg.TTSAudioEncoding),
		RawResponse: raw,
	}, nil
}

// SynthesizeSingle keeps the door open for future single-speaker podcast or other features.
func (c *Client) SynthesizeSingle(ctx context.Context, req SynthesizeSingleRequest) (AudioResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return AudioResult{}, errors.New("gemini single-speaker text is required")
	}
	voiceID := strings.TrimSpace(req.VoiceID)
	if voiceID == "" {
		return AudioResult{}, errors.New("gemini single-speaker voice id is required")
	}

	body := map[string]any{
		"input": map[string]any{
			"text": strings.TrimSpace(req.Text),
		},
		"voice": map[string]any{
			"languageCode": normalizeLanguageCode(req.LanguageCode),
			"name":         voiceID,
			"modelName":    c.cfg.TTSModel,
		},
		"audioConfig": map[string]any{
			"audioEncoding":   strings.ToUpper(strings.TrimSpace(c.cfg.TTSAudioEncoding)),
			"sampleRateHertz": c.cfg.TTSSampleRateHz,
		},
	}
	if speakingRate := firstPositiveFloat(req.SpeakingRate, c.cfg.TTSSpeakingRate); speakingRate > 0 {
		body["audioConfig"].(map[string]any)["speakingRate"] = speakingRate
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["input"].(map[string]any)["prompt"] = prompt
	}

	var resp struct {
		AudioContent string `json:"audioContent"`
	}
	raw, err := c.doJSONWithRetry(ctx, http.MethodPost, c.cfg.TTSURL, body, &resp)
	if err != nil {
		return AudioResult{}, err
	}
	if strings.TrimSpace(resp.AudioContent) == "" {
		return AudioResult{}, errors.New("gemini tts returned empty audio_content")
	}
	audio, err := base64.StdEncoding.DecodeString(resp.AudioContent)
	if err != nil {
		return AudioResult{}, err
	}
	return AudioResult{
		Audio:       audio,
		Ext:         audioExtForEncoding(c.cfg.TTSAudioEncoding),
		RawResponse: raw,
	}, nil
}

func newTokenSource(cfg Config, httpClient *http.Client) (tokenSource, error) {
	if token := strings.TrimSpace(cfg.AccessToken); token != "" {
		return staticTokenSource{token: token}, nil
	}

	creds, err := loadServiceAccountCredentials(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(creds.TokenURI) == "" {
		creds.TokenURI = cfg.TokenURL
	}
	return &serviceAccountTokenSource{
		creds:      creds,
		tokenURL:   cfg.TokenURL,
		httpClient: httpClient,
	}, nil
}

func loadServiceAccountCredentials(cfg Config) (serviceAccountCredentials, error) {
	if raw := strings.TrimSpace(cfg.ServiceAccountJSON); raw != "" {
		var creds serviceAccountCredentials
		if err := json.Unmarshal([]byte(raw), &creds); err != nil {
			return serviceAccountCredentials{}, err
		}
		return validateCredentials(creds)
	}

	path := strings.TrimSpace(cfg.ServiceAccountPath)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	}
	if path == "" {
		return serviceAccountCredentials{}, errors.New("google service account credentials are required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return serviceAccountCredentials{}, err
	}
	var creds serviceAccountCredentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return serviceAccountCredentials{}, err
	}
	return validateCredentials(creds)
}

func validateCredentials(creds serviceAccountCredentials) (serviceAccountCredentials, error) {
	if strings.TrimSpace(creds.ClientEmail) == "" || strings.TrimSpace(creds.PrivateKey) == "" {
		return serviceAccountCredentials{}, errors.New("google service account client_email/private_key are required")
	}
	return creds, nil
}

func (s *serviceAccountTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.token) != "" && time.Until(s.expiry) > time.Minute {
		return s.token, nil
	}

	token, expiry, err := s.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	s.token = token
	s.expiry = expiry
	return token, nil
}

func (s *serviceAccountTokenSource) fetchToken(ctx context.Context) (string, time.Time, error) {
	privateKey, err := parsePrivateKey(s.creds.PrivateKey)
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now().UTC()
	expiry := now.Add(55 * time.Minute)
	claims := map[string]any{
		"iss":   s.creds.ClientEmail,
		"scope": cloudPlatformScope,
		"aud":   firstNonEmpty(strings.TrimSpace(s.creds.TokenURI), strings.TrimSpace(s.tokenURL)),
		"iat":   now.Unix(),
		"exp":   expiry.Unix(),
	}
	assertion, err := signJWT(privateKey, claims)
	if err != nil {
		return "", time.Time{}, err
	}

	values := url.Values{}
	values.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	values.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, firstNonEmpty(strings.TrimSpace(s.creds.TokenURI), strings.TrimSpace(s.tokenURL)), strings.NewReader(values.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("google oauth failed status=%d body=%s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", time.Time{}, err
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return "", time.Time{}, errors.New("google oauth returned empty access_token")
	}
	tokenExpiry := now.Add(time.Duration(maxInt(parsed.ExpiresIn, 300)) * time.Second)
	return parsed.AccessToken, tokenExpiry, nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("google private key pem decode failed")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("google private key is not rsa")
		}
		return rsaKey, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func signJWT(privateKey *rsa.PrivateKey, claims map[string]any) (string, error) {
	headerRaw, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := rawURLEncode(headerRaw)
	encodedClaims := rawURLEncode(claimsRaw)
	unsigned := encodedHeader + "." + encodedClaims

	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + rawURLEncode(signature), nil
}

func rawURLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func (c *Client) doJSON(ctx context.Context, method, requestURL string, body any, out any) ([]byte, error) {
	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if userProject := strings.TrimSpace(c.cfg.UserProject); userProject != "" {
		req.Header.Set("x-goog-user-project", userProject)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("google api failed status=%d body=%s", resp.StatusCode, string(raw))
		if isNonRetryableStatus(resp.StatusCode) {
			return raw, services.NonRetryableError{Err: err}
		}
		if isRetryableStatus(resp.StatusCode) {
			return raw, retryableGoogleError{err: err}
		}
		return raw, err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, err
		}
	}
	return raw, nil
}

func (c *Client) doJSONWithRetry(ctx context.Context, method, requestURL string, body any, out any) ([]byte, error) {
	attempts := len(ttsRequestRetryDelays) + 1
	var lastRaw []byte
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		lastRaw, lastErr = c.doJSON(ctx, method, requestURL, body, out)
		if lastErr == nil {
			return lastRaw, nil
		}
		if !isRetryableGoogleRequestError(lastErr) || attempt == attempts {
			return lastRaw, lastErr
		}

		delay := ttsRequestRetryDelays[attempt-1]
		log.Printf("🔁 google tts request retry attempt=%d/%d delay=%s error=%v", attempt, attempts, delay.String(), lastErr)
		if err := sleepWithContext(ctx, delay); err != nil {
			return lastRaw, lastErr
		}
	}

	return lastRaw, lastErr
}

type retryableGoogleError struct {
	err error
}

func (e retryableGoogleError) Error() string {
	if e.err == nil {
		return "retryable google api error"
	}
	return e.err.Error()
}

func (e retryableGoogleError) Unwrap() error {
	return e.err
}

func isNonRetryableStatus(status int) bool {
	switch status {
	case http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableGoogleRequestError(err error) bool {
	if err == nil {
		return false
	}
	var permanent services.NonRetryableError
	if errors.As(err, &permanent) {
		return false
	}
	var retryable retryableGoogleError
	if errors.As(err, &retryable) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "tls handshake timeout") ||
		strings.Contains(message, "unexpected eof") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "temporarily unavailable")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func audioExtForEncoding(encoding string) string {
	switch strings.ToUpper(strings.TrimSpace(encoding)) {
	case "OGG_OPUS":
		return "ogg"
	case "LINEAR16", "PCM":
		return "wav"
	default:
		return "mp3"
	}
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeLanguageCode(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn":
		return "cmn-CN"
	case "ja", "ja-jp":
		return "ja-JP"
	default:
		return strings.TrimSpace(language)
	}
}

func normalizeSpeaker(speaker string) string {
	normalized := strings.ToLower(strings.TrimSpace(speaker))
	switch normalized {
	case "female", "f", "woman", "girl", "女":
		return "female"
	case "male", "m", "man", "boy", "男":
		return "male"
	}
	if strings.Contains(normalized, "female") || strings.Contains(normalized, "女") {
		return "female"
	}
	return "male"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
