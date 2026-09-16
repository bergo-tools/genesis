package llm

import (
	"context"
	"errors"
	"io"
	"strings"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

// MaxSpeechBytes caps a single synthesized audio response.
const MaxSpeechBytes = 25 << 20

// SpeechRequest is a text-to-speech request.
type SpeechRequest struct {
	Model  string
	Voice  string
	Input  string
	Format string
	Speed  float64
}

// SpeechResult is synthesized audio plus its MIME type.
type SpeechResult struct {
	Audio       []byte
	ContentType string
}

// SpeechClient is implemented by backends able to synthesize speech.
type SpeechClient interface {
	Speech(ctx context.Context, req SpeechRequest) (*SpeechResult, error)
}

// Speech synthesizes audio for the given text through OpenRouter's
// /audio/speech endpoint.
func (c *OpenRouterClient) Speech(ctx context.Context, req SpeechRequest) (*SpeechResult, error) {
	if c == nil || c.sdk == nil {
		return nil, errors.New("llm: openrouter client is not configured")
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		return nil, errors.New("llm: speech input is empty")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("llm: speech model is not configured")
	}
	format := normalizeSpeechFormat(req.Format)
	sr := components.SpeechRequest{
		Input:          input,
		Model:          req.Model,
		ResponseFormat: components.SpeechRequestResponseFormat(format).ToPointer(),
	}
	if v := strings.TrimSpace(req.Voice); v != "" {
		sr.Voice = openrouter.Pointer(v)
	}
	if req.Speed > 0 {
		sr.Speed = openrouter.Pointer(req.Speed)
	}
	stream, err := c.sdk.TTS.CreateSpeech(ctx, sr)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, MaxSpeechBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("llm: speech provider returned no audio")
	}
	if len(data) > MaxSpeechBytes {
		return nil, errors.New("llm: speech response too large")
	}
	return &SpeechResult{Audio: data, ContentType: speechContentType(format)}, nil
}

// SpeechExtension returns the file extension for a speech format.
func SpeechExtension(format string) string {
	return "." + normalizeSpeechFormat(format)
}

// SpeechContentType returns the MIME type for a speech format.
func SpeechContentType(format string) string {
	return speechContentType(normalizeSpeechFormat(format))
}

func normalizeSpeechFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3", "mpeg":
		return "mp3"
	case "wav", "wave":
		return "wav"
	case "opus", "ogg":
		return "opus"
	case "aac":
		return "aac"
	case "flac":
		return "flac"
	case "pcm":
		return "pcm"
	default:
		return "mp3"
	}
}

func speechContentType(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "pcm":
		return "application/octet-stream"
	default:
		return "audio/mpeg"
	}
}
