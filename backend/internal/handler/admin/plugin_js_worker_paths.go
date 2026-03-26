package admin

import (
	"path/filepath"

	"auralogic/internal/service"
)

func normalizeStoredJSWorkerPathConfig(packagePath string, address string) (string, string, error) {
	normalizedPackagePath, err := service.NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		return "", "", err
	}

	normalizedAddress, err := service.NormalizeJSWorkerRelativeEntryPath(normalizedPackagePath, address)
	if err != nil {
		return "", "", err
	}

	return filepath.ToSlash(normalizedPackagePath), normalizedAddress, nil
}
