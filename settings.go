package busylib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	httpKeyPattern       = regexp.MustCompile(`^[0-9]{4,10}$`)
	accessTokenIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)
)

// SettingsService reads and changes device-wide settings.
type SettingsService struct {
	client *Client
}

// Settings returns the device settings API.
func (c *Client) Settings() SettingsService { return SettingsService{client: c} }

// HTTPAccessInfo reports the device HTTP access mode and key state.
type HTTPAccessInfo struct {
	Mode     HTTPAccessMode `json:"mode"`
	KeyValid bool           `json:"key_valid"`
}

// StoredAccessToken identifies an access token without exposing its secret.
type StoredAccessToken struct {
	// ShortID is the first eight token characters and identifies the token for
	// revocation.
	ShortID string `json:"short_id"`
	// DisplayID is a redacted identifier formed from the token prefix and suffix.
	DisplayID string `json:"display_id"`
	// Name is the caller-supplied label for the token.
	Name string `json:"name"`
	// CreatedAt is a Unix millisecond timestamp encoded as a decimal string.
	CreatedAt string `json:"created_at"`
	// LastUsedAt is the latest-use Unix millisecond timestamp encoded as a
	// decimal string. The firmware reports "0" before the token is first used.
	LastUsedAt string `json:"last_used_at"`
}

// MintedAccessToken contains metadata for a newly created token and its
// one-time credential.
type MintedAccessToken struct {
	StoredAccessToken
	// Token is the full credential. The device returns it only when the token is
	// created.
	Token string `json:"token"`
}

// AccessTokensInfo contains the access tokens stored on the device.
type AccessTokensInfo struct {
	Tokens []StoredAccessToken `json:"tokens"`
}

// HTTPAccessMode controls access to the device HTTP API.
type HTTPAccessMode string

const (
	// HTTPAccessDisabled blocks HTTP API access.
	HTTPAccessDisabled HTTPAccessMode = "disabled"
	// HTTPAccessEnabled allows HTTP API access without a key.
	HTTPAccessEnabled HTTPAccessMode = "enabled"
	// HTTPAccessKey requires a valid access key.
	HTTPAccessKey HTTPAccessMode = "key"
)

// NameInfo contains the device name.
type NameInfo struct {
	Name string `json:"name"`
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

func validateDeviceName(name string) error {
	if len(name) < 1 || len(name) > 20 {
		return errors.New("device name must be 1-20 characters")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("device name must not contain only spaces")
	}
	const punctuation = " !()_=+;:,.?'|@#$%^&*[]{} /\\\"<>-"
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		if r > 0x7e || r < 0x20 || !strings.ContainsRune(punctuation, r) {
			return errors.New("device name contains unsupported characters")
		}
	}
	return nil
}

func validateHTTPAccess(mode HTTPAccessMode, key string) error {
	if mode != HTTPAccessDisabled && mode != HTTPAccessEnabled && mode != HTTPAccessKey {
		return fmt.Errorf("mode %q is not supported", mode)
	}
	if mode == HTTPAccessKey && !httpKeyPattern.MatchString(key) {
		return errors.New("key mode requires a 4-10 digit key")
	}
	if mode != HTTPAccessKey && key != "" && !httpKeyPattern.MatchString(key) {
		return errors.New("access key must be 4-10 digits when provided")
	}
	return nil
}

func validateAccessTokenShortID(shortID string) error {
	if !accessTokenIDPattern.MatchString(shortID) {
		return errors.New("access token short ID must contain exactly 8 ASCII letters or digits")
	}
	return nil
}
