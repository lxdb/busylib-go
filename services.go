package busylib

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type SystemService struct {
	client *Client
}

type SettingsService struct {
	client *Client
}

type DisplayService struct {
	client *Client
}

type AudioService struct {
	client *Client
}

type AssetsService struct {
	client *Client
}

type StorageService struct {
	client *Client
}

type BusyService struct {
	client *Client
}

type AccountService struct {
	client *Client
}

type BLEService struct {
	client *Client
}

type WiFiService struct {
	client *Client
}

type InputService struct {
	client *Client
}

type SmartHomeService struct {
	client *Client
}

type TimeService struct {
	client *Client
}

type UpdateService struct {
	client *Client
}

func (c *Client) System() SystemService       { return SystemService{client: c} }
func (c *Client) Settings() SettingsService   { return SettingsService{client: c} }
func (c *Client) Display() DisplayService     { return DisplayService{client: c} }
func (c *Client) Audio() AudioService         { return AudioService{client: c} }
func (c *Client) Assets() AssetsService       { return AssetsService{client: c} }
func (c *Client) Storage() StorageService     { return StorageService{client: c} }
func (c *Client) Busy() BusyService           { return BusyService{client: c} }
func (c *Client) Account() AccountService     { return AccountService{client: c} }
func (c *Client) BLE() BLEService             { return BLEService{client: c} }
func (c *Client) WiFi() WiFiService           { return WiFiService{client: c} }
func (c *Client) Input() InputService         { return InputService{client: c} }
func (c *Client) SmartHome() SmartHomeService { return SmartHomeService{client: c} }
func (c *Client) Time() TimeService           { return TimeService{client: c} }
func (c *Client) Update() UpdateService       { return UpdateService{client: c} }

func (s SystemService) Version(ctx context.Context) (VersionInfo, error) {
	var out VersionInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/version", nil, nil, &out)
	return out, err
}

func (s SystemService) Status(ctx context.Context) (Status, error) {
	var out Status
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &out)
	return out, err
}

func (s SystemService) DeviceStatus(ctx context.Context) (StatusDevice, error) {
	var out StatusDevice
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/device", nil, nil, &out)
	return out, err
}

func (s SystemService) FirmwareStatus(ctx context.Context) (StatusFirmware, error) {
	var out StatusFirmware
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/firmware", nil, nil, &out)
	return out, err
}

func (s SystemService) SystemStatus(ctx context.Context) (StatusSystem, error) {
	var out StatusSystem
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/system", nil, nil, &out)
	return out, err
}

func (s SystemService) PowerStatus(ctx context.Context) (StatusPower, error) {
	var out StatusPower
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/power", nil, nil, &out)
	return out, err
}

func (s SystemService) Transport(ctx context.Context) (NetworkInterfaceInfo, error) {
	var out NetworkInterfaceInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/transport", nil, nil, &out)
	return out, err
}

func (s SystemService) DumpLog(ctx context.Context, path string) error {
	if path != "" {
		if err := validateStoragePath("path", path); err != nil {
			return validationError(http.MethodPost, "/api/log_dump", err.Error(), err)
		}
	}
	query := url.Values{}
	if path != "" {
		query.Set("path", path)
	}
	return s.client.doTextSuccess(ctx, http.MethodPost, "/api/log_dump", query, nil)
}

func (s SettingsService) HTTPAccess(ctx context.Context) (HttpAccessInfo, error) {
	var out HttpAccessInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/access", nil, nil, &out)
	return out, err
}

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

func (s SettingsService) Name(ctx context.Context) (NameInfo, error) {
	var out NameInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/name", nil, nil, &out)
	return out, err
}

func (s SettingsService) SetName(ctx context.Context, name string) error {
	if err := validateDeviceName(name); err != nil {
		return validationError(http.MethodPost, "/api/name", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/name", nil, JSONBody(NameInfo{Name: name}))
}

func (s DisplayService) Brightness(ctx context.Context) (DisplayBrightnessInfo, error) {
	var out DisplayBrightnessInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/display/brightness", nil, nil, &out)
	return out, err
}

func (s DisplayService) SetBrightness(ctx context.Context, value string) error {
	if err := validateBrightness(value); err != nil {
		return validationError(http.MethodPost, "/api/display/brightness", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/brightness", url.Values{"value": []string{value}}, nil)
}

func (s DisplayService) Draw(ctx context.Context, request DisplayElements) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/display/draw", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/draw", nil, JSONBody(request))
}

func (s DisplayService) Clear(ctx context.Context, applicationName string) error {
	if applicationName != "" {
		if err := validateDisplayApplicationName(applicationName); err != nil {
			return validationError(http.MethodDelete, "/api/display/draw", err.Error(), err)
		}
	}
	query := url.Values{}
	if applicationName != "" {
		query.Set("application_name", applicationName)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/display/draw", query, nil)
}

func (s DisplayService) Screen(ctx context.Context, display int) ([]byte, error) {
	if err := validateScreenDisplay(display); err != nil {
		return nil, validationError(http.MethodGet, "/api/screen", err.Error(), err)
	}
	return s.client.doBytes(ctx, http.MethodGet, "/api/screen", url.Values{"display": []string{strconv.Itoa(display)}}, nil)
}

func (s AudioService) Play(ctx context.Context, request PlayAudio) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/audio/play", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/audio/play", nil, JSONBody(request))
}

func (s AudioService) PlayAsset(ctx context.Context, applicationName, path string) error {
	return s.Play(ctx, NewAssetAudio(applicationName, path))
}

func (s AudioService) PlayStock(ctx context.Context, applicationName, stockPath string) error {
	return s.Play(ctx, NewStockAudio(applicationName, stockPath))
}

func (s AudioService) Stop(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/audio/play", nil, nil)
}

func (s AudioService) Volume(ctx context.Context) (AudioVolumeInfo, error) {
	var out AudioVolumeInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/audio/volume", nil, nil, &out)
	return out, err
}

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

func (s AudioService) SetVolumeSilently(ctx context.Context, volume int) error {
	return s.SetVolume(ctx, SetAudioVolumeRequest{Volume: volume, Silent: true})
}

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

func (s AssetsService) UploadFile(ctx context.Context, applicationName, file, localPath string) error {
	return s.Upload(ctx, UploadAssetRequest{
		ApplicationName: applicationName,
		File:            file,
		Body:            FileBody(localPath, "application/octet-stream"),
	})
}

func (s AssetsService) DeleteApplicationAssets(ctx context.Context, applicationName string) error {
	if err := validateAssetParameter("application_name", applicationName); err != nil {
		return validationError(http.MethodDelete, "/api/assets/upload", err.Error(), err)
	}
	if !firmwarePathIsSane("/ext/user_assets/" + applicationName) {
		err := errors.New("application_name produces an unsafe firmware path")
		return validationError(http.MethodDelete, "/api/assets/upload", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/assets/upload", url.Values{"application_name": []string{applicationName}}, nil)
}

func (s StorageService) Write(ctx context.Context, request WriteStorageFileRequest) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/storage/write", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/write", url.Values{"path": []string{request.Path}}, request.Body)
}

func (s StorageService) WriteFile(ctx context.Context, path, localPath string) error {
	return s.Write(ctx, WriteStorageFileRequest{
		Path: path,
		Body: FileBody(localPath, "application/octet-stream"),
	})
}

func (s StorageService) Read(ctx context.Context, path string) ([]byte, error) {
	if err := validateStoragePath("path", path); err != nil {
		return nil, validationError(http.MethodGet, "/api/storage/read", err.Error(), err)
	}
	return s.client.doBytes(ctx, http.MethodGet, "/api/storage/read", url.Values{"path": []string{path}}, nil)
}

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

func (s StorageService) List(ctx context.Context, path string) (StorageList, error) {
	var out StorageList
	if err := validateStoragePath("path", path); err != nil {
		return out, validationError(http.MethodGet, "/api/storage/list", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/storage/list", url.Values{"path": []string{path}}, nil, &out)
	return out, err
}

func (s StorageService) Remove(ctx context.Context, path string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodDelete, "/api/storage/remove", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/storage/remove", url.Values{"path": []string{path}}, nil)
}

func (s StorageService) Mkdir(ctx context.Context, path string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodPost, "/api/storage/mkdir", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/mkdir", url.Values{"path": []string{path}}, nil)
}

func (s StorageService) Rename(ctx context.Context, path, newPath string) error {
	if err := validateStoragePath("path", path); err != nil {
		return validationError(http.MethodPost, "/api/storage/rename", err.Error(), err)
	}
	if err := validateStoragePath("new_path", newPath); err != nil {
		return validationError(http.MethodPost, "/api/storage/rename", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/storage/rename", url.Values{"path": []string{path}, "new_path": []string{newPath}}, nil)
}

func (s StorageService) Status(ctx context.Context) (StorageStatus, error) {
	var out StorageStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/storage/status", nil, nil, &out)
	return out, err
}

func (s BusyService) Snapshot(ctx context.Context) (BusySnapshot, error) {
	var out BusySnapshot
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/snapshot", nil, nil, &out)
	return out, err
}

func (s BusyService) SetSnapshot(ctx context.Context, snapshot BusySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/snapshot", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/snapshot", nil, JSONBody(snapshot))
}

func (s BusyService) Profile(ctx context.Context, slot BusyProfileSlot) (BusyProfile, error) {
	var out BusyProfile
	if err := validateBusyProfileSlot(slot); err != nil {
		return out, validationError(http.MethodGet, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, nil, &out)
	return out, err
}

func (s BusyService) SetProfile(ctx context.Context, slot BusyProfileSlot, profile BusyProfile) error {
	if err := validateBusyProfileSlot(slot); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	if err := profile.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, JSONBody(profile))
}

func (s AccountService) Status(ctx context.Context) (AccountStatus, error) {
	var out AccountStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/status", nil, nil, &out)
	return out, err
}

func (s AccountService) Info(ctx context.Context) (AccountInfo, error) {
	var out AccountInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/info", nil, nil, &out)
	return out, err
}

func (s AccountService) Link(ctx context.Context) (AccountLink, error) {
	var out AccountLink
	err := s.client.doJSON(ctx, http.MethodPost, "/api/account/link", nil, nil, &out)
	return out, err
}

func (s AccountService) Backend(ctx context.Context) (AccountBackend, error) {
	var out AccountBackend
	err := s.client.doJSON(ctx, http.MethodGet, "/api/account/backend", nil, nil, &out)
	return out, err
}

func (s AccountService) SetBackend(ctx context.Context, backend AccountBackend) error {
	if err := backend.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/account/backend", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/account/backend", nil, JSONBody(backend))
}

func (s AccountService) Unlink(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/account", nil, nil)
}

func (s BLEService) Status(ctx context.Context) (BleStatusResponse, error) {
	var out BleStatusResponse
	err := s.client.doJSON(ctx, http.MethodGet, "/api/ble/status", nil, nil, &out)
	return out, err
}

func (s BLEService) Enable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/enable", nil, nil)
}

func (s BLEService) Disable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/disable", nil, nil)
}

func (s BLEService) RemovePairing(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/ble/pairing", nil, nil)
}

func (s WiFiService) Status(ctx context.Context) (WiFiStatus, error) {
	var out WiFiStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/status", nil, nil, &out)
	return out, err
}

func (s WiFiService) Networks(ctx context.Context) (NetworkResponse, error) {
	var out NetworkResponse
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/networks", nil, nil, &out)
	return out, err
}

func (s WiFiService) Connect(ctx context.Context, request ConnectRequestConfig) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/wifi/connect", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/connect", nil, JSONBody(request))
}

func (s WiFiService) Disconnect(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/disconnect", nil, nil)
}

func (s InputService) SendKey(ctx context.Context, key InputKey) error {
	if err := validateInputKey(key); err != nil {
		return validationError(http.MethodPost, "/api/input", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/input", url.Values{"key": []string{string(key)}}, nil)
}

func (s SmartHomeService) PairingStatus(ctx context.Context) (SmartHomePairingInfo, error) {
	var out SmartHomePairingInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

func (s SmartHomeService) StartPairing(ctx context.Context) (SmartHomePairingPayload, error) {
	var out SmartHomePairingPayload
	err := s.client.doJSON(ctx, http.MethodPost, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

func (s SmartHomeService) ForgetPairings(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/smart_home/pairing", nil, nil)
}

func (s SmartHomeService) SwitchState(ctx context.Context) (SmartHomeSwitchState, error) {
	var out SmartHomeSwitchState
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/switch", nil, nil, &out)
	return out, err
}

func (s SmartHomeService) SetSwitchState(ctx context.Context, update SmartHomeSwitchUpdate) error {
	if err := update.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/smart_home/switch", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/smart_home/switch", nil, JSONBody(update))
}

func (s TimeService) Now(ctx context.Context) (TimestampInfo, error) {
	var out TimestampInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time", nil, nil, &out)
	return out, err
}

func (s TimeService) SetTimestamp(ctx context.Context, timestamp string) error {
	if err := validateTimestamp(timestamp); err != nil {
		return validationError(http.MethodPost, "/api/time/timestamp", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timestamp", url.Values{"timestamp": []string{timestamp}}, nil)
}

func (s TimeService) Timezone(ctx context.Context) (TimezoneInfo, error) {
	var out TimezoneInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/timezone", nil, nil, &out)
	return out, err
}

func (s TimeService) SetTimezone(ctx context.Context, timezone string) error {
	if err := validateTimezone(timezone); err != nil {
		return validationError(http.MethodPost, "/api/time/timezone", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timezone", url.Values{"timezone": []string{timezone}}, nil)
}

func (s TimeService) Timezones(ctx context.Context) (TimezoneListResponse, error) {
	var out TimezoneListResponse
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/tzlist", nil, nil, &out)
	return out, err
}

func (s UpdateService) UploadPackage(ctx context.Context, body Body) error {
	if body == nil {
		return validationError(http.MethodPost, "/api/update", "firmware update body must not be nil", nil)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update", nil, body)
}

func (s UpdateService) Check(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/check", nil, nil)
}

func (s UpdateService) Status(ctx context.Context) (UpdateStatus, error) {
	var out UpdateStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/status", nil, nil, &out)
	return out, err
}

func (s UpdateService) Changelog(ctx context.Context, version string) (UpdateChangelog, error) {
	var out UpdateChangelog
	if err := validateUpdateVersion(version); err != nil {
		return out, validationError(http.MethodGet, "/api/update/changelog", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/changelog", url.Values{"version": []string{version}}, nil, &out)
	return out, err
}

func (s UpdateService) Install(ctx context.Context, version string) error {
	if err := validateUpdateVersion(version); err != nil {
		return validationError(http.MethodPost, "/api/update/install", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/install", url.Values{"version": []string{version}}, nil)
}

func (s UpdateService) AbortDownload(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/update/abort_download", nil, nil)
}

func (s UpdateService) Autoupdate(ctx context.Context) (AutoupdateSettings, error) {
	var out AutoupdateSettings
	err := s.client.doJSON(ctx, http.MethodGet, "/api/update/autoupdate", nil, nil, &out)
	return out, err
}

func (s UpdateService) SetAutoupdate(ctx context.Context, settings AutoupdateSettings) error {
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
	var out SuccessResponse
	return c.doJSON(ctx, method, path, query, body, &out)
}

func (c *Client) doTextSuccess(ctx context.Context, method, path string, query url.Values, body Body) error {
	_, err := c.Do(ctx, Request{
		Method:       method,
		Path:         path,
		Query:        query,
		Body:         body,
		ResponseMode: ResponseModeText,
	})
	return err
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
