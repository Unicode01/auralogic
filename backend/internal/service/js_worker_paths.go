package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	jsWorkerExtractNamespace = "pkg"
	jsWorkerExtractRootName  = "root"
)

func NormalizeJSWorkerPackagePath(packagePath string) (string, error) {
	trimmed := strings.TrimSpace(packagePath)
	if trimmed == "" {
		return "", fmt.Errorf("js_worker package path is empty")
	}

	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if !filepath.IsAbs(cleaned) {
		absPath, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("resolve js_worker package path: %w", err)
		}
		cleaned = absPath
	}
	return filepath.Clean(cleaned), nil
}

func ResolveJSWorkerPackageRoot(packagePath string) (string, error) {
	packageAbsPath, err := NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(packageAbsPath)
	if statErr == nil && info.IsDir() {
		return packageAbsPath, nil
	}
	if statErr == nil {
		derivedRoot := jsWorkerDerivedExtractRoot(packageAbsPath)
		if derivedInfo, derivedErr := os.Stat(derivedRoot); derivedErr == nil && derivedInfo.IsDir() {
			return derivedRoot, nil
		}
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Errorf("stat js_worker package path: %w", statErr)
	}

	if isJSWorkerPackageFile(packageAbsPath) {
		return jsWorkerDerivedExtractRoot(packageAbsPath), nil
	}
	return packageAbsPath, nil
}

func NormalizeJSWorkerRelativeEntryPath(packagePath string, address string) (string, error) {
	root, err := ResolveJSWorkerPackageRoot(packagePath)
	if err != nil {
		return "", err
	}

	rootInfo, statErr := os.Stat(root)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return "", fmt.Errorf("js_worker plugin root not found: %s", filepath.ToSlash(root))
		}
		return "", fmt.Errorf("stat js_worker plugin root: %w", statErr)
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("js_worker plugin root is not a directory: %s", filepath.ToSlash(root))
	}

	resolvedPath, relPath, err := resolveJSWorkerEntryWithinRoot(root, address)
	if err != nil {
		return "", err
	}

	entryInfo, statErr := os.Stat(resolvedPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return "", fmt.Errorf("js_worker entry script not found: %s", filepath.ToSlash(resolvedPath))
		}
		return "", fmt.Errorf("stat js_worker entry script: %w", statErr)
	}
	if entryInfo.IsDir() {
		return "", fmt.Errorf("js_worker entry script is a directory: %s", filepath.ToSlash(resolvedPath))
	}

	return filepath.ToSlash(relPath), nil
}

func ResolveJSWorkerScriptPath(address string, packagePath string) (string, error) {
	trimmedAddress := strings.TrimSpace(address)
	if trimmedAddress == "" {
		return "", fmt.Errorf("js_worker entry script path is empty")
	}

	if strings.TrimSpace(packagePath) == "" {
		return resolveLegacyJSWorkerAbsolutePath(trimmedAddress)
	}

	root, rootErr := ResolveJSWorkerPackageRoot(packagePath)
	if rootErr == nil {
		rootInfo, statErr := os.Stat(root)
		if statErr == nil && rootInfo.IsDir() {
			resolvedPath, _, resolveErr := resolveJSWorkerEntryWithinRoot(root, trimmedAddress)
			if resolveErr == nil {
				entryInfo, entryErr := os.Stat(resolvedPath)
				if entryErr == nil && !entryInfo.IsDir() {
					return resolvedPath, nil
				}
				if entryErr != nil && !os.IsNotExist(entryErr) {
					return "", fmt.Errorf("stat js_worker entry script: %w", entryErr)
				}
			}
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return "", fmt.Errorf("stat js_worker plugin root: %w", statErr)
		}
	}

	if resolvedPath, ok := resolveLegacyAllowedAbsoluteScriptPath(trimmedAddress, packagePath); ok {
		return resolvedPath, nil
	}

	if rootErr != nil {
		return "", rootErr
	}
	return "", fmt.Errorf("js_worker entry script not found: %s", trimmedAddress)
}

func resolveJSWorkerEntryWithinRoot(root string, address string) (string, string, error) {
	trimmedAddress := strings.TrimSpace(address)
	if trimmedAddress == "" {
		return "", "", fmt.Errorf("js_worker entry script path is empty")
	}

	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", "", fmt.Errorf("resolve js_worker plugin root: %w", err)
		}
		root = filepath.Clean(absRoot)
	}

	cleanedAddress := filepath.Clean(filepath.FromSlash(trimmedAddress))
	var resolvedPath string
	if filepath.IsAbs(cleanedAddress) {
		resolvedPath = cleanedAddress
	} else {
		if cleanedAddress == "." || cleanedAddress == "" {
			return "", "", fmt.Errorf("js_worker entry script path is empty")
		}
		resolvedPath = filepath.Join(root, cleanedAddress)
	}
	resolvedPath = filepath.Clean(resolvedPath)

	if !isPathWithinRoot(root, resolvedPath) {
		return "", "", fmt.Errorf("js_worker entry is outside plugin root")
	}

	if realPath, evalErr := filepath.EvalSymlinks(resolvedPath); evalErr == nil {
		resolvedPath = filepath.Clean(realPath)
		if !isPathWithinRoot(root, resolvedPath) {
			return "", "", fmt.Errorf("js_worker entry symlink resolves outside plugin root")
		}
	}

	relPath, err := filepath.Rel(root, resolvedPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve js_worker entry relative path: %w", err)
	}
	relPath = filepath.Clean(relPath)
	if relPath == "." || relPath == "" {
		return "", "", fmt.Errorf("js_worker entry script path is empty")
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("js_worker entry is outside plugin root")
	}

	return resolvedPath, relPath, nil
}

func resolveLegacyJSWorkerAbsolutePath(address string) (string, error) {
	resolvedPath, ok := normalizeExistingAbsoluteJSWorkerPath(address)
	if !ok {
		return "", fmt.Errorf("js_worker package_path is required for relative entry paths")
	}
	return resolvedPath, nil
}

func resolveLegacyAllowedAbsoluteScriptPath(address string, packagePath string) (string, bool) {
	resolvedPath, ok := normalizeExistingAbsoluteJSWorkerPath(address)
	if !ok {
		return "", false
	}

	if strings.TrimSpace(packagePath) == "" {
		return resolvedPath, true
	}

	packageAbsPath, err := NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		return "", false
	}

	info, statErr := os.Stat(packageAbsPath)
	if statErr == nil && info.IsDir() {
		if isPathWithinRoot(packageAbsPath, resolvedPath) {
			return resolvedPath, true
		}
		return "", false
	}

	derivedRoot := jsWorkerDerivedExtractRoot(packageAbsPath)
	if isPathWithinRoot(derivedRoot, resolvedPath) {
		return resolvedPath, true
	}

	legacyRoot := legacyJSWorkerArtifactRoot(packageAbsPath)
	if legacyRoot != "" && isPathWithinRoot(legacyRoot, resolvedPath) {
		return resolvedPath, true
	}

	return "", false
}

func normalizeExistingAbsoluteJSWorkerPath(address string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(address)))
	if !filepath.IsAbs(cleaned) {
		return "", false
	}

	info, err := os.Stat(cleaned)
	if err != nil || info.IsDir() {
		return "", false
	}
	return cleaned, true
}

func isJSWorkerPackageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".zip", ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func jsWorkerDerivedExtractRoot(packageAbsPath string) string {
	baseName := strings.TrimSuffix(filepath.Base(packageAbsPath), filepath.Ext(packageAbsPath))
	safeBaseName := sanitizeJSWorkerPathSegment(baseName)
	return filepath.Clean(filepath.Join(
		filepath.Dir(packageAbsPath),
		"jsworker",
		jsWorkerExtractNamespace,
		safeBaseName,
		jsWorkerExtractRootName,
	))
}

func legacyJSWorkerArtifactRoot(packageAbsPath string) string {
	if strings.TrimSpace(packageAbsPath) == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(packageAbsPath), "jsworker"))
}

func sanitizeJSWorkerPathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "plugin"
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, ch := range trimmed {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		case ch == '.', ch == '-', ch == '_':
			builder.WriteRune(ch)
		default:
			builder.WriteByte('_')
		}
	}

	out := strings.Trim(builder.String(), "._-")
	if out == "" {
		return "plugin"
	}
	return out
}

func isPathWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
