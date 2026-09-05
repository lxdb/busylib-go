package busylib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxStoragePathBytes = 63

// StorageService manages files in device storage.
type StorageService struct {
	client *Client
}

// Storage returns the device file storage API.
func (c *Client) Storage() StorageService { return StorageService{client: c} }

// WriteStorageFileRequest writes a body to a device storage path. A nil Append
// value and a pointer to false both select replacement behavior. A non-nil
// value is sent to the firmware so callers can distinguish an omitted option
// from append=0.
type WriteStorageFileRequest struct {
	Path   string
	Body   Body
	Append *bool
}

// StorageList contains the entries in a device storage directory.
type StorageList struct {
	List []StorageListElement `json:"list"`
}

// StorageListElement describes one file or directory.
type StorageListElement struct {
	Type StorageListElementType `json:"type"`
	Name string                 `json:"name"`
	Size uint64                 `json:"size,omitempty"`
}

// StorageListElementType identifies a file-system entry type.
type StorageListElementType string

const (
	// StorageListElementFile identifies a regular file.
	StorageListElementFile StorageListElementType = "file"
	// StorageListElementDir identifies a directory.
	StorageListElementDir StorageListElementType = "dir"
)

// StorageStatus reports device storage capacity and use.
type StorageStatus struct {
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
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

// Validate reports whether a storage write has a safe path and a body.
func (request WriteStorageFileRequest) Validate() error {
	if err := validateStoragePath("path", request.Path); err != nil {
		return err
	}
	if request.Body == nil {
		return errors.New("storage write body must not be nil")
	}
	return nil
}

func validateStoragePath(field, value string) error {
	if value == "" || len(value) > maxStoragePathBytes {
		return fmt.Errorf("%s must be 1-%d bytes", field, maxStoragePathBytes)
	}
	trimmed := strings.TrimSuffix(value, "/")
	if !strings.HasPrefix(trimmed, "/ext") || !firmwarePathIsSane(trimmed) {
		return fmt.Errorf("%s must use a sane firmware path under /ext", field)
	}
	return nil
}

func firmwarePathIsSane(value string) bool {
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, "..") {
		return false
	}
	return !strings.Contains(value, "/..") && !strings.Contains(value, `\..`)
}
