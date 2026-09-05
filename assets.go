package busylib

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// AssetsService uploads and removes application assets.
type AssetsService struct {
	client *Client
}

// Assets returns the application asset API.
func (c *Client) Assets() AssetsService { return AssetsService{client: c} }

// UploadAssetRequest stores Body as File under ApplicationName.
type UploadAssetRequest struct {
	ApplicationName string
	File            string
	Body            Body
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

// Validate reports whether an asset upload has safe names and a body.
func (request UploadAssetRequest) Validate() error {
	if err := validateApplicationName(request.ApplicationName); err != nil {
		return err
	}
	if err := validateUploadedAssetPath("file", request.File); err != nil {
		return err
	}
	if request.Body == nil {
		return errors.New("asset upload body must not be nil")
	}
	return nil
}
