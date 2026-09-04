package busylib

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// SystemService reads device identity, health, transport, and diagnostic data.
type SystemService struct {
	client *Client
}

// SettingsService reads and changes device-wide settings.
type SettingsService struct {
	client *Client
}

// DisplayService controls brightness and rendered display content.
type DisplayService struct {
	client *Client
}

// AudioService controls playback and output volume.
type AudioService struct {
	client *Client
}

// AssetsService uploads and removes application assets.
type AssetsService struct {
	client *Client
}

// StorageService manages files in device storage.
type StorageService struct {
	client *Client
}

// BusyService reads and changes busy-state snapshots and profiles.
type BusyService struct {
	client *Client
}

// AccountService manages remote account links and backend settings.
type AccountService struct {
	client *Client
}

// BLEService reads and controls Bluetooth Low Energy state and pairing.
type BLEService struct {
	client *Client
}

// WiFiService reads network state and controls Wi-Fi connections.
type WiFiService struct {
	client *Client
}

// InputService sends virtual key input to the device.
type InputService struct {
	client *Client
}

// SmartHomeService controls smart-home pairing and switch state.
type SmartHomeService struct {
	client *Client
}

// TimeService reads and changes the device clock and timezone.
type TimeService struct {
	client *Client
}

// UpdateService checks, uploads, installs, and stops firmware updates.
type UpdateService struct {
	client *Client
}

// System returns the device system API.
func (c *Client) System() SystemService { return SystemService{client: c} }

// Settings returns the device settings API.
func (c *Client) Settings() SettingsService { return SettingsService{client: c} }

// Display returns the display control API.
func (c *Client) Display() DisplayService { return DisplayService{client: c} }

// Audio returns the audio control API.
func (c *Client) Audio() AudioService { return AudioService{client: c} }

// Assets returns the application asset API.
func (c *Client) Assets() AssetsService { return AssetsService{client: c} }

// Storage returns the device file storage API.
func (c *Client) Storage() StorageService { return StorageService{client: c} }

// Busy returns the busy-state API.
func (c *Client) Busy() BusyService { return BusyService{client: c} }

// Account returns the remote account API.
func (c *Client) Account() AccountService { return AccountService{client: c} }

// BLE returns the Bluetooth Low Energy API.
func (c *Client) BLE() BLEService { return BLEService{client: c} }

// WiFi returns the Wi-Fi control API.
func (c *Client) WiFi() WiFiService { return WiFiService{client: c} }

// Input returns the virtual input API.
func (c *Client) Input() InputService { return InputService{client: c} }

// SmartHome returns the smart-home API.
func (c *Client) SmartHome() SmartHomeService { return SmartHomeService{client: c} }

// Time returns the device clock API.
func (c *Client) Time() TimeService { return TimeService{client: c} }

// Update returns the firmware update API.
func (c *Client) Update() UpdateService { return UpdateService{client: c} }

// Version returns device firmware and API version details.
func (s SystemService) Version(ctx context.Context) (VersionInfo, error) {
	var out VersionInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/version", nil, nil, &out)
	return out, err
}

// Status returns the aggregate device status.
func (s SystemService) Status(ctx context.Context) (Status, error) {
	var out Status
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &out)
	return out, err
}

// DeviceStatus returns identity and hardware status.
func (s SystemService) DeviceStatus(ctx context.Context) (DeviceStatus, error) {
	var out DeviceStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/device", nil, nil, &out)
	return out, err
}

// FirmwareStatus returns firmware version and build status.
func (s SystemService) FirmwareStatus(ctx context.Context) (FirmwareStatus, error) {
	var out FirmwareStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/firmware", nil, nil, &out)
	return out, err
}

// SystemStatus returns runtime and memory status.
func (s SystemService) SystemStatus(ctx context.Context) (SystemStatus, error) {
	var out SystemStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/system", nil, nil, &out)
	return out, err
}

// PowerStatus returns power-source and battery status.
func (s SystemService) PowerStatus(ctx context.Context) (PowerStatus, error) {
	var out PowerStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/power", nil, nil, &out)
	return out, err
}

// Transport returns the network interface used by the API.
func (s SystemService) Transport(ctx context.Context) (NetworkInterfaceInfo, error) {
	var out NetworkInterfaceInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/transport", nil, nil, &out)
	return out, err
}

// DumpLog creates a device log archive and returns its storage path.
// An empty filename lets the device choose the archive name.
func (s SystemService) DumpLog(ctx context.Context, filename string) (LogDumpResponse, error) {
	var out LogDumpResponse
	if err := validateOptionalLogFilename(filename); err != nil {
		return out, validationError(http.MethodPost, "/api/log_dump", err.Error(), err)
	}
	query := url.Values{}
	if filename != "" {
		query.Set("filename", filename)
	}
	err := s.client.doJSON(ctx, http.MethodPost, "/api/log_dump", query, nil, &out)
	return out, err
}

// HTTPAccess returns the local HTTP access mode.
func (s SettingsService) HTTPAccess(ctx context.Context) (HTTPAccessInfo, error) {
	var out HTTPAccessInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/access", nil, nil, &out)
	return out, err
}

// SetHTTPAccess changes the local HTTP access mode and optional numeric key.
// The change can invalidate the credentials used by subsequent requests.
func (s SettingsService) SetHTTPAccess(ctx context.Context, mode HTTPAccessMode, key string) error {
	if err := validateHTTPAccess(mode, key); err != nil {
		return validationError(http.MethodPost, "/api/access", err.Error(), err)
	}
	query := url.Values{"mode": []string{string(mode)}}
	if key != "" {
		query.Set("key", key)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/access", query, nil)
}

// AccessTokens returns information about access tokens stored on the device. It
// never returns their full credentials.
func (s SettingsService) AccessTokens(ctx context.Context) (AccessTokensInfo, error) {
	var out AccessTokensInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/access/tokens", nil, nil, &out)
	return out, err
}

// MintAccessToken creates an access token and returns its one-time credential.
// The device rejects this operation when the request uses an access token.
func (s SettingsService) MintAccessToken(ctx context.Context, name string) (MintedAccessToken, error) {
	var out MintedAccessToken
	err := s.client.doJSON(ctx, http.MethodPost, "/api/access/tokens", nil, JSONBody(NameInfo{Name: name}), &out)
	return out, err
}

// RevokeAccessToken removes the access token with the supplied short ID. A
// token-authenticated client can revoke only its own token.
func (s SettingsService) RevokeAccessToken(ctx context.Context, shortID string) error {
	const path = "/api/access/tokens/{short_id}"
	if err := validateAccessTokenShortID(shortID); err != nil {
		return validationError(http.MethodDelete, path, err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/access/tokens/"+shortID, nil, nil)
}

// RevokeAllAccessTokens removes every access token stored on the device. The
// device rejects this operation when the request uses an access token.
func (s SettingsService) RevokeAllAccessTokens(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/access/tokens", nil, nil)
}

// Name returns the device name.
func (s SettingsService) Name(ctx context.Context) (NameInfo, error) {
	var out NameInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/name", nil, nil, &out)
	return out, err
}

// SetName changes the device name after local validation.
func (s SettingsService) SetName(ctx context.Context, name string) error {
	if err := validateDeviceName(name); err != nil {
		return validationError(http.MethodPost, "/api/name", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/name", nil, JSONBody(NameInfo{Name: name}))
}

// Brightness returns the current brightness value or automatic mode.
func (s DisplayService) Brightness(ctx context.Context) (DisplayBrightnessInfo, error) {
	var out DisplayBrightnessInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/display/brightness", nil, nil, &out)
	return out, err
}

// SetBrightness selects automatic mode or a percentage from 0 through 100.
func (s DisplayService) SetBrightness(ctx context.Context, value string) error {
	if err := validateBrightness(value); err != nil {
		return validationError(http.MethodPost, "/api/display/brightness", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/brightness", url.Values{"value": []string{value}}, nil)
}

// Draw validates and renders application elements on the device displays.
func (s DisplayService) Draw(ctx context.Context, request DisplayElements) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/display/draw", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/draw", nil, JSONBody(request))
}

// Clear removes rendered elements for one application.
// An empty application name removes all rendered elements.
func (s DisplayService) Clear(ctx context.Context, applicationName string) error {
	if applicationName != "" {
		if err := validateApplicationName(applicationName); err != nil {
			return validationError(http.MethodDelete, "/api/display/draw", err.Error(), err)
		}
	}
	query := url.Values{}
	if applicationName != "" {
		query.Set("application_name", applicationName)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/display/draw", query, nil)
}

// ClearElements removes selected rendered elements. A non-empty ApplicationName
// limits removal to that application. An empty ApplicationName applies the
// ElementIDs filter across all applications. ApplicationName is sent as a query
// parameter because firmware 1.2.3 ignores it in the request body.
//
// WARNING: Firmware 1.2.3 has an unterminated internal element_ids pointer
// array. This operation may behave incorrectly or restart the device.
func (s DisplayService) ClearElements(ctx context.Context, request ClearDisplayElementsRequest) error {
	if err := validateClearDisplayElementsRequest(request); err != nil {
		return validationError(http.MethodDelete, "/api/display/draw", err.Error(), err)
	}
	query := url.Values{}
	if request.ApplicationName != "" {
		query.Set("application_name", request.ApplicationName)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/display/draw", query, JSONBody(request))
}

// Screen returns the uncompressed pixels of the selected display. Pass the
// result to frame.FromHTTP when metadata or image conversion is required.
func (s DisplayService) Screen(ctx context.Context, display DisplayTarget) ([]byte, error) {
	if err := validateScreenDisplay(display); err != nil {
		return nil, validationError(http.MethodGet, "/api/screen", err.Error(), err)
	}
	displayNumber := 0
	if display == DisplayBack {
		displayNumber = 1
	}
	response, err := s.client.Do(ctx, Request{
		Method:       http.MethodGet,
		Path:         "/api/screen",
		Query:        url.Values{"display": []string{strconv.Itoa(displayNumber)}},
		ResponseMode: ResponseModeBytes,
	})
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(response.Body))
	if err != nil {
		return nil, &ProtocolError{
			Method:    http.MethodGet,
			Path:      "/api/screen",
			RequestID: response.RequestID,
			Excerpt:   excerpt(response.Body),
			Err:       err,
		}
	}
	return decoded, nil
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

// Upload stores one application asset from the supplied body.
func (s AssetsService) Upload(ctx context.Context, request UploadAssetRequest) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/assets/upload", err.Error(), err)
	}
	query := url.Values{
		"application_name": []string{request.ApplicationName},
		"file":             []string{request.File},
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/assets/upload", query, request.Body)
}

// UploadFile stores a local file as an application asset.
func (s AssetsService) UploadFile(ctx context.Context, applicationName, file, localPath string) error {
	return s.Upload(ctx, UploadAssetRequest{
		ApplicationName: applicationName,
		File:            file,
		Body:            FileBody(localPath, "application/octet-stream"),
	})
}

// DeleteApplicationAssets permanently removes all assets owned by one
// application.
func (s AssetsService) DeleteApplicationAssets(ctx context.Context, applicationName string) error {
	if err := validateApplicationName(applicationName); err != nil {
		return validationError(http.MethodDelete, "/api/assets/upload", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/assets/upload", url.Values{"application_name": []string{applicationName}}, nil)
}

// Write stores content from the supplied body. Append nil omits the append
// option; non-nil values explicitly select append or replacement behavior.
func (s StorageService) Write(ctx context.Context, request WriteStorageFileRequest) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/storage/write", err.Error(), err)
	}
	query := url.Values{"path": []string{request.Path}}
	if request.Append != nil {
		appendValue := "0"
		if *request.Append {
			appendValue = "1"
		}
		query.Set("append", appendValue)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/write", query, request.Body)
}

// WriteFile uploads a local file to the selected device path.
func (s StorageService) WriteFile(ctx context.Context, path, localPath string) error {
	return s.Write(ctx, WriteStorageFileRequest{
		Path: path,
		Body: FileBody(localPath, "application/octet-stream"),
	})
}

// Read returns a complete device file in memory.
// The configured response-size limit applies to the file.
func (s StorageService) Read(ctx context.Context, path string) ([]byte, error) {
	if err := validateStoragePath("path", path); err != nil {
		return nil, validationError(http.MethodGet, "/api/storage/read", err.Error(), err)
	}
	return s.client.doBytes(ctx, http.MethodGet, "/api/storage/read", url.Values{"path": []string{path}}, nil)
}

// ReadTo streams a device file to writer and returns the bytes written.
func (s StorageService) ReadTo(ctx context.Context, path string, writer io.Writer) (int64, error) {
	if err := validateStoragePath("path", path); err != nil {
		return 0, validationError(http.MethodGet, "/api/storage/read", err.Error(), err)
	}
	if writer == nil {
		return 0, validationError(http.MethodGet, "/api/storage/read", "writer must not be nil", nil)
	}
	_, n, err := s.client.doStreamTo(ctx, Request{
		Method:       http.MethodGet,
		Path:         "/api/storage/read",
		Query:        url.Values{"path": []string{path}},
		ResponseMode: ResponseModeBytes,
	}, writer)
	return n, err
}

// List returns the entries below a device directory.
func (s StorageService) List(ctx context.Context, path string) (StorageList, error) {
	var out StorageList
	if err := validateStoragePath("path", path); err != nil {
		return out, validationError(http.MethodGet, "/api/storage/list", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/storage/list", url.Values{"path": []string{path}}, nil, &out)
	return out, err
}

// Remove permanently deletes a device file or directory.
func (s StorageService) Remove(ctx context.Context, path string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodDelete, "/api/storage/remove", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/storage/remove", url.Values{"path": []string{path}}, nil)
}

// Mkdir creates a directory at the device path.
func (s StorageService) Mkdir(ctx context.Context, path string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodPost, "/api/storage/mkdir", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/mkdir", url.Values{"path": []string{path}}, nil)
}

// Rename moves a device file or directory to newPath.
func (s StorageService) Rename(ctx context.Context, path, newPath string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodPost, "/api/storage/rename", err.Error(), err)
	}
	if err := validateStoragePath("new_path", newPath); err != nil {
		return validationError(http.MethodPost, "/api/storage/rename", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/rename", url.Values{"path": []string{path}, "new_path": []string{newPath}}, nil)
}

// Status returns device storage capacity and usage.
func (s StorageService) Status(ctx context.Context) (StorageStatus, error) {
	var out StorageStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/storage/status", nil, nil, &out)
	return out, err
}

// Snapshot returns the current busy-state snapshot.
func (s BusyService) Snapshot(ctx context.Context) (BusySnapshot, error) {
	var out BusySnapshot
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/snapshot", nil, nil, &out)
	return out, err
}

// SetSnapshot validates and replaces the complete current busy-state snapshot.
// Read Snapshot first when fields not owned by the caller must be preserved.
func (s BusyService) SetSnapshot(ctx context.Context, snapshot BusySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/snapshot", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/snapshot", nil, JSONBody(snapshot))
}

// Profile returns the saved busy profile in slot.
func (s BusyService) Profile(ctx context.Context, slot BusyProfileSlot) (BusyProfile, error) {
	var out BusyProfile
	if err := validateBusyProfileSlot(slot); err != nil {
		return out, validationError(http.MethodGet, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, nil, &out)
	return out, err
}

// SetProfile validates and replaces the complete saved busy profile in slot.
func (s BusyService) SetProfile(ctx context.Context, slot BusyProfileSlot, profile BusyProfile) error {
	if err := validateBusyProfileSlot(slot); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	if err := profile.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, JSONBody(profile))
}

// Status returns the remote account connection state.
func (s AccountService) Status(ctx context.Context) (AccountStatus, error) {
	var out AccountStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/status", nil, nil, &out)
	return out, err
}

// Info returns the linked remote account details.
func (s AccountService) Info(ctx context.Context) (AccountInfo, error) {
	var out AccountInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/info", nil, nil, &out)
	return out, err
}

// Link starts account linking and returns temporary user authorization details.
// It is unavailable through remote.Client.Device.
func (s AccountService) Link(ctx context.Context) (AccountLink, error) {
	var out AccountLink
	err := s.client.doJSON(ctx, http.MethodPost, "/api/account/link", nil, nil, &out)
	return out, err
}

// Backend returns the remote account backend configuration.
func (s AccountService) Backend(ctx context.Context) (AccountBackend, error) {
	var out AccountBackend
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/backend", nil, nil, &out)
	return out, err
}

// SetBackend validates and replaces the remote account backend configuration.
// It is unavailable through remote.Client.Device.
func (s AccountService) SetBackend(ctx context.Context, backend AccountBackend) error {
	if err := backend.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/account/backend", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/account/backend", nil, JSONBody(backend))
}

// Unlink disconnects the device from its remote account. It is unavailable
// through remote.Client.Device.
func (s AccountService) Unlink(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/account", nil, nil)
}

// Status returns Bluetooth Low Energy state and pairing details.
func (s BLEService) Status(ctx context.Context) (BLEStatus, error) {
	var out BLEStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/ble/status", nil, nil, &out)
	return out, err
}

// Enable turns on Bluetooth Low Energy support.
func (s BLEService) Enable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/enable", nil, nil)
}

// Disable turns off Bluetooth Low Energy support.
func (s BLEService) Disable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/disable", nil, nil)
}

// RemovePairing removes the saved Bluetooth Low Energy pairing.
func (s BLEService) RemovePairing(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/ble/pairing", nil, nil)
}

// Status returns the current Wi-Fi connection details.
func (s WiFiService) Status(ctx context.Context) (WiFiStatus, error) {
	var out WiFiStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/status", nil, nil, &out)
	return out, err
}

// Networks scans for available Wi-Fi networks. It is unavailable through
// remote.Client.Device.
func (s WiFiService) Networks(ctx context.Context) (WiFiNetworkList, error) {
	var out WiFiNetworkList
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/networks", nil, nil, &out)
	return out, err
}

// Connect validates the network settings and starts a Wi-Fi connection. It is
// unavailable through remote.Client.Device and can replace the caller's active
// network path.
func (s WiFiService) Connect(ctx context.Context, request WiFiConnectRequest) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/wifi/connect", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/connect", nil, JSONBody(request))
}

// Disconnect stops the current Wi-Fi connection. It is unavailable through
// remote.Client.Device and can remove the caller's active network path.
func (s WiFiService) Disconnect(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/disconnect", nil, nil)
}

// SendKey sends one virtual key press to the device.
func (s InputService) SendKey(ctx context.Context, key InputKey) error {
	if err := validateInputKey(key); err != nil {
		return validationError(http.MethodPost, "/api/input", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/input", url.Values{"key": []string{string(key)}}, nil)
}

// PairingStatus returns the current smart-home pairing state.
func (s SmartHomeService) PairingStatus(ctx context.Context) (SmartHomePairingInfo, error) {
	var out SmartHomePairingInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

// StartPairing starts smart-home pairing and returns setup data.
func (s SmartHomeService) StartPairing(ctx context.Context) (SmartHomePairingPayload, error) {
	var out SmartHomePairingPayload
	err := s.client.doJSON(ctx, http.MethodPost, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

// ForgetPairings permanently removes all saved smart-home pairings.
func (s SmartHomeService) ForgetPairings(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/smart_home/pairing", nil, nil)
}

// SwitchState returns the current smart-home switch configuration.
func (s SmartHomeService) SwitchState(ctx context.Context) (SmartHomeSwitchState, error) {
	var out SmartHomeSwitchState
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/switch", nil, nil, &out)
	return out, err
}

// SetSwitchState validates and changes the supplied smart-home switch fields.
// Nil pointer fields preserve their current values.
func (s SmartHomeService) SetSwitchState(ctx context.Context, update SmartHomeSwitchUpdate) error {
	if err := update.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/smart_home/switch", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/smart_home/switch", nil, JSONBody(update))
}

// Now returns the device timestamp.
func (s TimeService) Now(ctx context.Context) (TimestampInfo, error) {
	var out TimestampInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time", nil, nil, &out)
	return out, err
}

// SetTimestamp changes the device clock from an RFC 3339 timestamp.
func (s TimeService) SetTimestamp(ctx context.Context, timestamp string) error {
	if err := validateTimestamp(timestamp); err != nil {
		return validationError(http.MethodPost, "/api/time/timestamp", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timestamp", url.Values{"timestamp": []string{timestamp}}, nil)
}

// Timezone returns the current device timezone.
func (s TimeService) Timezone(ctx context.Context) (TimezoneInfo, error) {
	var out TimezoneInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/timezone", nil, nil, &out)
	return out, err
}

// SetTimezone changes the device timezone.
func (s TimeService) SetTimezone(ctx context.Context, timezone string) error {
	if err := validateTimezone(timezone); err != nil {
		return validationError(http.MethodPost, "/api/time/timezone", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timezone", url.Values{"timezone": []string{timezone}}, nil)
}

// Timezones returns the timezone names supported by the device.
func (s TimeService) Timezones(ctx context.Context) (TimezoneListResponse, error) {
	var out TimezoneListResponse
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/tzlist", nil, nil, &out)
	return out, err
}

// UploadPackage uploads a firmware package from body. It is unavailable through
// remote.Client.Device.
func (s UpdateService) UploadPackage(ctx context.Context, body Body) error {
	if body == nil {
		return validationError(http.MethodPost, "/api/update", "firmware update body must not be nil", nil)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update", nil, body)
}

// Check asks the device to check for available firmware updates.
func (s UpdateService) Check(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/check", nil, nil)
}

// Status returns the current firmware update state.
func (s UpdateService) Status(ctx context.Context) (UpdateStatus, error) {
	var out UpdateStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/status", nil, nil, &out)
	return out, err
}

// Changelog returns release notes for a firmware version.
func (s UpdateService) Changelog(ctx context.Context, version string) (UpdateChangelog, error) {
	var out UpdateChangelog
	if err := validateUpdateVersion(version); err != nil {
		return out, validationError(http.MethodGet, "/api/update/changelog", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/changelog", url.Values{"version": []string{version}}, nil, &out)
	return out, err
}

// Install starts downloading and installing a firmware version. A successful
// response means the device accepted the operation; use Status to observe it.
func (s UpdateService) Install(ctx context.Context, version string) error {
	if err := validateUpdateVersion(version); err != nil {
		return validationError(http.MethodPost, "/api/update/install", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/install", url.Values{"version": []string{version}}, nil)
}

// AbortDownload stops the active firmware download.
func (s UpdateService) AbortDownload(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/abort_download", nil, nil)
}

// AutoUpdate returns the device's current automatic-update settings.
func (s UpdateService) AutoUpdate(ctx context.Context) (AutoUpdateSettings, error) {
	var out AutoUpdateSettings
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/autoupdate", nil, nil, &out)
	return out, err
}

// SetAutoUpdate validates and applies automatic-update settings. A nil
// IsEnabled field preserves the current enabled state.
func (s UpdateService) SetAutoUpdate(ctx context.Context, settings AutoUpdateSettings) error {
	if err := settings.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/update/autoupdate", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/autoupdate", nil, JSONBody(settings))
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body Body, target any) error {
	response, err := c.Do(ctx, Request{
		Method:       method,
		Path:         path,
		Query:        query,
		Body:         body,
		ResponseMode: ResponseModeJSON,
	})
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	return response.DecodeJSON(target)
}

func (c *Client) doSuccess(ctx context.Context, method, path string, query url.Values, body Body) error {
	var out successResponse
	return c.doJSON(ctx, method, path, query, body, &out)
}

func (c *Client) doBytes(ctx context.Context, method, path string, query url.Values, body Body) ([]byte, error) {
	response, err := c.Do(ctx, Request{
		Method:       method,
		Path:         path,
		Query:        query,
		Body:         body,
		ResponseMode: ResponseModeBytes,
	})
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
