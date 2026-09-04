package busylib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const maxAccountServerURLBytes = 64

// AccountService manages remote account links and backend settings.
type AccountService struct {
	client *Client
}

// Account returns the remote account API.
func (c *Client) Account() AccountService { return AccountService{client: c} }

// AccountLink contains a temporary account-link code and its firmware-provided
// expiry time. Treat Code as authorization data.
type AccountLink struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

// AccountInfo reports the account linked to the device. Email and user IDs can
// contain private account data.
type AccountInfo struct {
	Linked bool   `json:"linked"`
	ID     string `json:"id"`
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

// AccountStatus reports the connection state of the linked account.
type AccountStatus struct {
	Status AccountConnectionStatus `json:"status"`
}

// AccountConnectionStatus identifies the account backend connection state.
type AccountConnectionStatus string

const (
	// AccountStatusError means the account connection failed.
	AccountStatusError AccountConnectionStatus = "error"
	// AccountStatusDisconnected means no account connection is active.
	AccountStatusDisconnected AccountConnectionStatus = "disconnected"
	// AccountStatusConnected means the account connection is active.
	AccountStatusConnected AccountConnectionStatus = "connected"
)

// AccountBackend configures the account server and client certificate mode.
// IgnoreServerCert asks the firmware to ignore server-certificate validation;
// use it only with an explicitly trusted development endpoint.
type AccountBackend struct {
	ServerURL        string                `json:"server_url"`
	ClientCertType   AccountClientCertType `json:"client_cert_type"`
	IgnoreServerCert bool                  `json:"ignore_server_cert"`
}

// AccountClientCertType selects the certificate used for account requests.
type AccountClientCertType string

const (
	// AccountClientCertDefault uses the firmware default certificate.
	AccountClientCertDefault AccountClientCertType = "default"
	// AccountClientCertCustom uses a user-provided certificate.
	AccountClientCertCustom AccountClientCertType = "custom"
	// AccountClientCertNone sends no client certificate.
	AccountClientCertNone AccountClientCertType = "none"
)

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

// Validate reports whether remote account backend settings are syntactically
// valid for the device API.
func (backend AccountBackend) Validate() error {
	if strings.TrimSpace(backend.ServerURL) == "" {
		return errors.New("server_url must not be empty")
	}
	if len(backend.ServerURL) > maxAccountServerURLBytes {
		return fmt.Errorf("server_url must be at most %d bytes", maxAccountServerURLBytes)
	}
	if backend.ServerURL != "default" &&
		!strings.HasPrefix(backend.ServerURL, "mqtt://") &&
		!strings.HasPrefix(backend.ServerURL, "mqtts://") {
		return errors.New("server_url must be default, mqtt://, or mqtts://")
	}
	if !validAccountClientCertType(backend.ClientCertType) {
		return fmt.Errorf("client_cert_type %q is not supported", backend.ClientCertType)
	}
	return nil
}

func validAccountClientCertType(value AccountClientCertType) bool {
	switch value {
	case AccountClientCertDefault, AccountClientCertCustom, AccountClientCertNone:
		return true
	default:
		return false
	}
}
