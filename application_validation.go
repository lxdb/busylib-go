package busylib

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	applicationNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	assetPathPattern       = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

const (
	maxApplicationNameBytes   = 31
	maxUploadedAssetPathBytes = 64
	maxStockAssetPathBytes    = 256
)

func validateAssetSource(field, applicationName, path, stockPath string) error {
	if err := validateApplicationName(applicationName); err != nil {
		return err
	}
	hasPath := path != ""
	hasStockPath := stockPath != ""
	if hasPath == hasStockPath {
		return fmt.Errorf("%s must set exactly one of path or stock_path", field)
	}
	if hasPath {
		return validateUploadedAssetPath(field+".path", path)
	}
	return validateStockAssetPath(field+".stock_path", stockPath)
}

func validateApplicationName(value string) error {
	if len(value) < 1 || len(value) > maxApplicationNameBytes {
		return fmt.Errorf("application_name must be 1-%d bytes", maxApplicationNameBytes)
	}
	if !applicationNamePattern.MatchString(value) {
		return errors.New("application_name must contain only ASCII letters, digits, periods, underscores, or hyphens")
	}
	return nil
}

func validateUploadedAssetPath(field, value string) error {
	return validateSafeAssetPath(field, value, maxUploadedAssetPathBytes)
}

func validateStockAssetPath(field, value string) error {
	if err := validateSafeAssetPath(field, value, maxStockAssetPathBytes); err != nil {
		return err
	}
	if !strings.HasPrefix(value, "shared/") {
		return fmt.Errorf("%s must begin with shared/", field)
	}
	return nil
}

func validateSafeAssetPath(field, value string, maximumBytes int) error {
	if len(value) < 1 || len(value) > maximumBytes {
		return fmt.Errorf("%s must be 1-%d bytes", field, maximumBytes)
	}
	if !assetPathPattern.MatchString(value) {
		return fmt.Errorf("%s contains unsupported characters", field)
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") || strings.Contains(value, "..") {
		return fmt.Errorf("%s must be a safe relative asset path", field)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." {
			return fmt.Errorf("%s must not contain traversal segments", field)
		}
	}
	return nil
}
