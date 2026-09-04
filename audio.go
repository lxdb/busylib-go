package busylib

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// AudioService controls playback and output volume.
type AudioService struct {
	client *Client
}

// Audio returns the audio control API.
func (c *Client) Audio() AudioService { return AudioService{client: c} }

// PlayAudio selects exactly one uploaded Path or firmware StockPath for
// playback.
type PlayAudio struct {
	ApplicationName string `json:"application_name"`
	Path            string `json:"path,omitempty"`
	StockPath       string `json:"stock_path,omitempty"`
}

// AudioVolumeInfo contains the current playback volume from 0 through 100.
type AudioVolumeInfo struct {
	Volume int `json:"volume"`
}

// SetAudioVolumeRequest changes playback volume from 0 through 100. Silent
// suppresses the device's feedback sound.
type SetAudioVolumeRequest struct {
	Volume int
	Silent bool
}

// NewAssetAudio creates a playback request for an uploaded asset.
func NewAssetAudio(applicationName, path string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		Path:            path,
	}
}

// NewStockAudio creates a playback request for a firmware stock asset.
func NewStockAudio(applicationName, stockPath string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		StockPath:       stockPath,
	}
}

// Play starts playback of one uploaded or stock audio asset.
func (s AudioService) Play(ctx context.Context, request PlayAudio) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/audio/play", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/audio/play", nil, JSONBody(request))
}

// PlayAsset starts playback of an uploaded application asset.
func (s AudioService) PlayAsset(ctx context.Context, applicationName, path string) error {
	return s.Play(ctx, NewAssetAudio(applicationName, path))
}

// PlayStock starts playback of a firmware stock asset.
func (s AudioService) PlayStock(ctx context.Context, applicationName, stockPath string) error {
	return s.Play(ctx, NewStockAudio(applicationName, stockPath))
}

// Stop stops the current audio playback.
func (s AudioService) Stop(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/audio/play", nil, nil)
}

// Volume returns the current output volume.
func (s AudioService) Volume(ctx context.Context) (AudioVolumeInfo, error) {
	var out AudioVolumeInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/audio/volume", nil, nil, &out)
	return out, err
}

// SetVolume changes the output volume and can suppress device feedback.
func (s AudioService) SetVolume(ctx context.Context, request SetAudioVolumeRequest) error {
	if err := validateVolume(request.Volume); err != nil {
		return validationError(http.MethodPost, "/api/audio/volume", err.Error(), err)
	}
	query := url.Values{"volume": []string{strconv.Itoa(request.Volume)}}
	if request.Silent {
		query.Set("silent", "1")
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/audio/volume", query, nil)
}

// SetVolumeSilently changes the output volume without device feedback.
func (s AudioService) SetVolumeSilently(ctx context.Context, volume int) error {
	return s.SetVolume(ctx, SetAudioVolumeRequest{Volume: volume, Silent: true})
}

// Validate reports whether an audio request selects one valid asset source.
func (request PlayAudio) Validate() error {
	return validateAssetSource("audio", request.ApplicationName, request.Path, request.StockPath)
}

func validateVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return errors.New("volume must be between 0 and 100")
	}
	return nil
}
