package busylib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

const maxUpdateVersionBytes = 63

// UpdateService checks, uploads, installs, and stops firmware updates.
type UpdateService struct {
	client *Client
}

// Update returns the firmware update API.
func (c *Client) Update() UpdateService { return UpdateService{client: c} }

// UpdateStatus reports firmware update installation and availability checks.
type UpdateStatus struct {
	Install UpdateInstallStatus `json:"install"`
	Check   UpdateCheckStatus   `json:"check"`
}

// UpdateInstallStatus reports the current firmware installation step and result.
type UpdateInstallStatus struct {
	IsAllowed bool                 `json:"is_allowed"`
	Event     UpdateInstallEvent   `json:"event"`
	Action    UpdateInstallAction  `json:"action"`
	Status    UpdateInstallResult  `json:"status"`
	Detail    string               `json:"detail,omitempty"`
	Download  UpdateDownloadStatus `json:"download"`
}

// UpdateDownloadStatus reports firmware download progress.
type UpdateDownloadStatus struct {
	SpeedBytesPerSec int64 `json:"speed_bytes_per_sec"`
	ReceivedBytes    int64 `json:"received_bytes"`
	TotalBytes       int64 `json:"total_bytes"`
}

// UpdateCheckStatus reports the latest firmware availability check.
type UpdateCheckStatus struct {
	AvailableVersion string            `json:"available_version"`
	Event            UpdateCheckEvent  `json:"event"`
	Status           UpdateCheckResult `json:"status"`
}

// UpdateInstallEvent identifies a change in the installation lifecycle.
type UpdateInstallEvent string

const (
	// UpdateInstallEventSessionStart begins an installation session.
	UpdateInstallEventSessionStart UpdateInstallEvent = "session_start"
	// UpdateInstallEventSessionStop ends an installation session.
	UpdateInstallEventSessionStop UpdateInstallEvent = "session_stop"
	// UpdateInstallEventActionBegin begins one installation action.
	UpdateInstallEventActionBegin UpdateInstallEvent = "action_begin"
	// UpdateInstallEventActionDone completes one installation action.
	UpdateInstallEventActionDone UpdateInstallEvent = "action_done"
	// UpdateInstallEventDetailChange updates installation detail text.
	UpdateInstallEventDetailChange UpdateInstallEvent = "detail_change"
	// UpdateInstallEventActionProgress updates installation progress.
	UpdateInstallEventActionProgress UpdateInstallEvent = "action_progress"
	// UpdateInstallEventNone means no installation event is active.
	UpdateInstallEventNone UpdateInstallEvent = "none"
)

// UpdateInstallAction identifies the active firmware installation step.
type UpdateInstallAction string

const (
	// UpdateInstallActionDownload downloads the firmware package.
	UpdateInstallActionDownload UpdateInstallAction = "download"
	// UpdateInstallActionSHAVerification verifies the package digest.
	UpdateInstallActionSHAVerification UpdateInstallAction = "sha_verification"
	// UpdateInstallActionUnpack extracts the firmware package.
	UpdateInstallActionUnpack UpdateInstallAction = "unpack"
	// UpdateInstallActionPrepare prepares the installation target.
	UpdateInstallActionPrepare UpdateInstallAction = "prepare"
	// UpdateInstallActionApply applies the prepared firmware.
	UpdateInstallActionApply UpdateInstallAction = "apply"
	// UpdateInstallActionNone means no installation action is active.
	UpdateInstallActionNone UpdateInstallAction = "none"
)

// UpdateInstallResult identifies the firmware installation result.
type UpdateInstallResult string

const (
	// UpdateInstallOK means installation succeeded.
	UpdateInstallOK UpdateInstallResult = "ok"
	// UpdateInstallBatteryLow means the battery charge is too low.
	UpdateInstallBatteryLow UpdateInstallResult = "battery_low"
	// UpdateInstallBusy means another operation blocks installation.
	UpdateInstallBusy UpdateInstallResult = "busy"
	// UpdateInstallDownloadFailure means the package download failed.
	UpdateInstallDownloadFailure UpdateInstallResult = "download_failure"
	// UpdateInstallDownloadAbort means the package download was canceled.
	UpdateInstallDownloadAbort UpdateInstallResult = "download_abort"
	// UpdateInstallSHAMismatch means package digest verification failed.
	UpdateInstallSHAMismatch UpdateInstallResult = "sha_mismatch"
	// UpdateInstallUnpackStagingDirFailure means staging directory creation failed.
	UpdateInstallUnpackStagingDirFailure UpdateInstallResult = "unpack_staging_dir_failure"
	// UpdateInstallUnpackArchiveOpenFailure means the firmware archive could not open.
	UpdateInstallUnpackArchiveOpenFailure UpdateInstallResult = "unpack_archive_open_failure"
	// UpdateInstallUnpackArchiveUnpackFailure means archive extraction failed.
	UpdateInstallUnpackArchiveUnpackFailure UpdateInstallResult = "unpack_archive_unpack_failure"
	// UpdateInstallManifestNotFound means the package has no install manifest.
	UpdateInstallManifestNotFound UpdateInstallResult = "install_manifest_not_found"
	// UpdateInstallManifestInvalid means the install manifest is invalid.
	UpdateInstallManifestInvalid UpdateInstallResult = "install_manifest_invalid"
	// UpdateInstallSessionConfigFailure means session configuration failed.
	UpdateInstallSessionConfigFailure UpdateInstallResult = "install_session_config_failure"
	// UpdateInstallPointerSetupFailure means installation pointer setup failed.
	UpdateInstallPointerSetupFailure UpdateInstallResult = "install_pointer_setup_failure"
	// UpdateInstallUnknownFailure means installation failed for an unknown reason.
	UpdateInstallUnknownFailure UpdateInstallResult = "unknown_failure"
)

// UpdateCheckEvent identifies a firmware availability check lifecycle event.
type UpdateCheckEvent string

const (
	// UpdateCheckEventStart begins an availability check.
	UpdateCheckEventStart UpdateCheckEvent = "start"
	// UpdateCheckEventStop ends an availability check.
	UpdateCheckEventStop UpdateCheckEvent = "stop"
	// UpdateCheckEventNone means no availability check event is active.
	UpdateCheckEventNone UpdateCheckEvent = "none"
)

// UpdateCheckResult identifies the result of a firmware availability check.
type UpdateCheckResult string

const (
	// UpdateCheckAvailable means a firmware update is available.
	UpdateCheckAvailable UpdateCheckResult = "available"
	// UpdateCheckNotAvailable means the current firmware is up to date.
	UpdateCheckNotAvailable UpdateCheckResult = "not_available"
	// UpdateCheckFailure means the availability check failed.
	UpdateCheckFailure UpdateCheckResult = "failure"
	// UpdateCheckNone means no availability check result exists.
	UpdateCheckNone UpdateCheckResult = "none"
)

// UpdateChangelog contains the changelog for an available firmware update.
type UpdateChangelog struct {
	Changelog string `json:"changelog"`
}

// AutoUpdateSettings configures automatic updates and their daily time window.
// IntervalStart and IntervalEnd use 24-hour HH:MM values. Nil IsEnabled leaves
// the current enabled state unchanged when updating settings.
type AutoUpdateSettings struct {
	IsEnabled     *bool  `json:"is_enabled,omitempty"`
	IntervalStart string `json:"interval_start,omitempty"`
	IntervalEnd   string `json:"interval_end,omitempty"`
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

// Validate reports whether automatic update settings use a supported schedule.
func (settings AutoUpdateSettings) Validate() error {
	if settings.IntervalStart != "" && !validFirmwareClock(settings.IntervalStart) {
		return errors.New("interval_start must contain an hour from 0-23 and minute from 0-59")
	}
	if settings.IntervalEnd != "" && !validFirmwareClock(settings.IntervalEnd) {
		return errors.New("interval_end must contain an hour from 0-23 and minute from 0-59")
	}
	return nil
}

func validateUpdateVersion(version string) error {
	if version == "" || len(version) > maxUpdateVersionBytes {
		return fmt.Errorf("version must be 1-%d bytes", maxUpdateVersionBytes)
	}
	return nil
}

func validFirmwareClock(value string) bool {
	var hour, minute int
	count, err := fmt.Sscanf(value, "%d:%d", &hour, &minute)
	return err == nil && count == 2 && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}
