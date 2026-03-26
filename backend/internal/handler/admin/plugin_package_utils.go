package admin

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/service"
)

func unzipPackageSafe(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open package zip: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create extract dir: %w", err)
	}

	rootClean := filepath.Clean(destDir)
	var fileCount int
	var declaredTotalBytes uint64
	var extractedTotalBytes int64
	for _, file := range reader.File {
		fileCount++
		if fileCount > maxPluginPackageFiles {
			return fmt.Errorf("zip contains too many entries (max=%d)", maxPluginPackageFiles)
		}

		targetPath := filepath.Clean(filepath.Join(destDir, filepath.FromSlash(file.Name)))
		if !isPathWithinRoot(rootClean, targetPath) {
			return fmt.Errorf("invalid zip entry path: %s", file.Name)
		}

		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry contains symlink: %s", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry contains symlink: %s", file.Name)
		}

		if !file.FileInfo().IsDir() {
			if file.UncompressedSize64 > uint64(maxPluginPackageSingleFileBytes) {
				return fmt.Errorf("zip entry %s exceeds single-file limit", file.Name)
			}
			declaredTotalBytes += file.UncompressedSize64
			if declaredTotalBytes > uint64(maxPluginPackageTotalBytes) {
				return fmt.Errorf("zip declared uncompressed size exceeds limit")
			}
			if file.CompressedSize64 > 0 {
				ratio := float64(file.UncompressedSize64) / float64(file.CompressedSize64)
				if ratio > maxPluginPackageCompressionRatio {
					return fmt.Errorf("zip entry %s compression ratio too high", file.Name)
				}
			} else if file.UncompressedSize64 > 0 {
				return fmt.Errorf("zip entry %s has invalid compressed size", file.Name)
			}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("create folder failed for %s: %w", file.Name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("create parent folder failed for %s: %w", file.Name, err)
		}

		src, openErr := file.Open()
		if openErr != nil {
			return fmt.Errorf("open zip entry %s failed: %w", file.Name, openErr)
		}

		dst, createErr := os.Create(targetPath)
		if createErr != nil {
			_ = src.Close()
			return fmt.Errorf("create target file %s failed: %w", targetPath, createErr)
		}

		limitReader := io.LimitReader(src, maxPluginPackageSingleFileBytes+1)
		written, copyErr := io.Copy(dst, limitReader)
		if closeErr := dst.Close(); closeErr != nil {
			_ = src.Close()
			_ = os.Remove(targetPath)
			return fmt.Errorf("close target file %s failed: %w", targetPath, closeErr)
		}
		_ = src.Close()
		if copyErr != nil {
			_ = os.Remove(targetPath)
			return fmt.Errorf("extract zip entry %s failed: %w", file.Name, copyErr)
		}
		if written > maxPluginPackageSingleFileBytes {
			_ = os.Remove(targetPath)
			return fmt.Errorf("zip entry %s exceeds single-file limit", file.Name)
		}
		extractedTotalBytes += written
		if extractedTotalBytes > maxPluginPackageTotalBytes {
			_ = os.Remove(targetPath)
			return fmt.Errorf("zip extracted size exceeds total limit")
		}

		if file.Mode()&0111 != 0 {
			if chmodErr := os.Chmod(targetPath, 0644); chmodErr != nil {
				_ = os.Remove(targetPath)
				return fmt.Errorf("sanitize file mode failed for %s: %w", targetPath, chmodErr)
			}
		}

		if _, statErr := os.Stat(targetPath); statErr != nil {
			_ = os.Remove(targetPath)
			return fmt.Errorf("validate extracted file %s failed: %w", targetPath, statErr)
		}
	}
	return nil
}

func findFirstJSFile(root string) (string, error) {
	var first string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if isSupportedJSWorkerEntryFile(path) {
			first = path
			return io.EOF
		}
		return nil
	})
	if errors.Is(err, io.EOF) && first != "" {
		return first, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("no supported js_worker entry found in package")
}

func findDefaultJSWorkerEntryFile(root string) string {
	for _, ext := range []string{".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx"} {
		candidate := filepath.Join(root, "index"+ext)
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func isSupportedJSWorkerEntryFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isPathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	parentPrefix := ".." + string(os.PathSeparator)
	return !strings.HasPrefix(rel, parentPrefix)
}

func (h *PluginHandler) collectPluginArtifactTargets(plugin *models.Plugin, versions []models.PluginVersion) ([]string, []string) {
	uploadRoot := strings.TrimSpace(h.uploadDir)
	if uploadRoot == "" {
		uploadRoot = filepath.Join("uploads", "plugins")
	}
	uploadRootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(uploadRoot)))
	if err != nil {
		return nil, nil
	}

	fileSet := make(map[string]struct{})
	dirSet := make(map[string]struct{})

	if plugin != nil {
		addArtifactPackagePath(fileSet, uploadRootAbs, plugin.PackagePath)
		addArtifactAddressPath(fileSet, dirSet, uploadRootAbs, plugin.Address, plugin.PackagePath)
		if plugin.ID > 0 {
			if dataRoot := pluginDataLayerRoot(uploadRootAbs, plugin.ID); dataRoot != "" {
				dirSet[dataRoot] = struct{}{}
			}
		}
	}
	for i := range versions {
		addArtifactPackagePath(fileSet, uploadRootAbs, versions[i].PackagePath)
		addArtifactAddressPath(fileSet, dirSet, uploadRootAbs, versions[i].Address, versions[i].PackagePath)
	}

	return sortedArtifactTargets(fileSet, dirSet)
}

func (h *PluginHandler) collectPluginVersionArtifactTargets(plugin *models.Plugin, version *models.PluginVersion, siblingVersions []models.PluginVersion) ([]string, []string) {
	if version == nil {
		return nil, nil
	}

	uploadRoot := strings.TrimSpace(h.uploadDir)
	if uploadRoot == "" {
		uploadRoot = filepath.Join("uploads", "plugins")
	}
	uploadRootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(uploadRoot)))
	if err != nil {
		return nil, nil
	}

	referencedFiles := make(map[string]struct{})
	referencedDirs := make(map[string]struct{})
	if plugin != nil {
		addArtifactPackagePath(referencedFiles, uploadRootAbs, plugin.PackagePath)
		addArtifactAddressPath(referencedFiles, referencedDirs, uploadRootAbs, plugin.Address, plugin.PackagePath)
	}
	for i := range siblingVersions {
		addArtifactPackagePath(referencedFiles, uploadRootAbs, siblingVersions[i].PackagePath)
		addArtifactAddressPath(referencedFiles, referencedDirs, uploadRootAbs, siblingVersions[i].Address, siblingVersions[i].PackagePath)
	}

	fileSet := make(map[string]struct{})
	dirSet := make(map[string]struct{})
	if resolved, ok := normalizeArtifactPathWithinUploadRoot(uploadRootAbs, version.PackagePath); ok {
		if _, exists := referencedFiles[resolved]; !exists {
			fileSet[resolved] = struct{}{}
		}
	}
	if resolved, ok := normalizeArtifactAddressWithinUploadRoot(uploadRootAbs, version.Address, version.PackagePath); ok {
		if extractRoot := resolveJSWorkerExtractRoot(uploadRootAbs, resolved); extractRoot != "" {
			if _, exists := referencedDirs[extractRoot]; !exists {
				dirSet[extractRoot] = struct{}{}
			}
		} else if strings.EqualFold(filepath.Ext(resolved), ".js") {
			if _, exists := referencedFiles[resolved]; !exists {
				fileSet[resolved] = struct{}{}
			}
		}
	}

	return sortedArtifactTargets(fileSet, dirSet)
}

func addArtifactPackagePath(fileSet map[string]struct{}, uploadRootAbs, path string) {
	resolved, ok := normalizeArtifactPathWithinUploadRoot(uploadRootAbs, path)
	if !ok {
		return
	}
	fileSet[resolved] = struct{}{}
}

func addArtifactAddressPath(fileSet, dirSet map[string]struct{}, uploadRootAbs, path string, packagePath string) {
	resolved, ok := normalizeArtifactAddressWithinUploadRoot(uploadRootAbs, path, packagePath)
	if !ok {
		return
	}
	if extractRoot := resolveJSWorkerExtractRoot(uploadRootAbs, resolved); extractRoot != "" {
		dirSet[extractRoot] = struct{}{}
		return
	}
	if strings.EqualFold(filepath.Ext(resolved), ".js") {
		fileSet[resolved] = struct{}{}
	}
}

func normalizeArtifactAddressWithinUploadRoot(uploadRootAbs, address, packagePath string) (string, bool) {
	resolvedPath := resolveArtifactScriptPath(address, packagePath)
	if strings.TrimSpace(resolvedPath) == "" {
		return "", false
	}
	return normalizeArtifactPathWithinUploadRoot(uploadRootAbs, resolvedPath)
}

func sortedArtifactTargets(fileSet, dirSet map[string]struct{}) ([]string, []string) {
	files := make([]string, 0, len(fileSet))
	for path := range fileSet {
		files = append(files, path)
	}
	dirs := make([]string, 0, len(dirSet))
	for path := range dirSet {
		dirs = append(dirs, path)
	}

	sort.Slice(files, func(i, j int) bool { return len(files[i]) > len(files[j]) })
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	return files, dirs
}

func normalizeArtifactPathWithinUploadRoot(uploadRootAbs, rawPath string) (string, bool) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", false
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", false
		}
		cleaned = abs
	}
	cleaned = filepath.Clean(cleaned)
	if cleaned == uploadRootAbs {
		return "", false
	}
	if !isPathWithinRoot(uploadRootAbs, cleaned) {
		return "", false
	}
	return cleaned, true
}

func resolveJSWorkerExtractRoot(uploadRootAbs, artifactPath string) string {
	rel, err := filepath.Rel(uploadRootAbs, artifactPath)
	if err != nil {
		return ""
	}
	parts := splitPathSegments(rel)
	for i, part := range parts {
		if !strings.EqualFold(part, "jsworker") {
			continue
		}
		if len(parts) < i+4 {
			return ""
		}
		root := filepath.Join(append([]string{uploadRootAbs}, parts[:i+4]...)...)
		root = filepath.Clean(root)
		if root == uploadRootAbs || !isPathWithinRoot(uploadRootAbs, root) {
			return ""
		}
		return root
	}
	return ""
}

func splitPathSegments(path string) []string {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == "" {
		return nil
	}
	rawParts := strings.Split(cleaned, string(os.PathSeparator))
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "." {
			continue
		}
		if trimmed == ".." {
			return nil
		}
		parts = append(parts, trimmed)
	}
	return parts
}

func pluginDataLayerRoot(uploadRootAbs string, pluginID uint) string {
	if pluginID == 0 {
		return ""
	}
	root := filepath.Clean(filepath.Join(uploadRootAbs, "data", fmt.Sprintf("plugin_%d", pluginID)))
	if root == uploadRootAbs || !isPathWithinRoot(uploadRootAbs, root) {
		return ""
	}
	return root
}

func cleanupPluginArtifactTargets(fileTargets, dirTargets []string) []string {
	cleanupErrors := make([]string, 0)
	for _, dir := range dirTargets {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("remove directory %s failed: %v", filepath.ToSlash(dir), err))
			continue
		}
		pruneEmptyParentDirs(filepath.Dir(dir))
	}
	for _, file := range fileTargets {
		if err := os.Remove(file); err == nil || os.IsNotExist(err) {
			if err == nil {
				pruneEmptyParentDirs(filepath.Dir(file))
			}
			continue
		}
		if fallbackErr := os.RemoveAll(file); fallbackErr != nil && !os.IsNotExist(fallbackErr) {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("remove file %s failed: %v", filepath.ToSlash(file), fallbackErr))
			continue
		}
		pruneEmptyParentDirs(filepath.Dir(file))
	}
	return cleanupErrors
}

func pruneEmptyParentDirs(start string) {
	current := filepath.Clean(start)
	for current != "" && current != "." {
		parent := filepath.Dir(current)
		if parent == current {
			return
		}
		if err := os.Remove(current); err != nil {
			if os.IsNotExist(err) {
				current = parent
				continue
			}
			return
		}
		current = parent
	}
}

type jsWorkerWritableMigrationResult struct {
	Applied      bool
	Reason       string
	SourceRoot   string
	TargetRoot   string
	FilesCopied  int
	FilesSkipped int
	BytesCopied  int64
}

func (h *PluginHandler) migrateJSWorkerWritableFilesForActivation(current *models.Plugin, next *models.PluginVersion) (jsWorkerWritableMigrationResult, error) {
	result := jsWorkerWritableMigrationResult{}
	if current == nil || next == nil {
		result.Reason = "plugin_or_version_nil"
		return result, nil
	}

	runtimeName := strings.ToLower(strings.TrimSpace(firstNonEmpty(next.Runtime, current.Runtime)))
	if runtimeName != service.PluginRuntimeJSWorker {
		result.Reason = "runtime_not_js_worker"
		return result, nil
	}
	if current.ID == 0 {
		result.Reason = "plugin_id_empty"
		return result, nil
	}

	sourceScript := resolveArtifactScriptPath(current.Address, current.PackagePath)
	if strings.TrimSpace(sourceScript) == "" {
		result.Reason = "script_path_empty"
		return result, nil
	}

	sourceRoot := detectArtifactModuleRoot(sourceScript)
	result.SourceRoot = filepath.ToSlash(sourceRoot)
	if sourceRoot == "" {
		result.Reason = "module_root_empty"
		return result, nil
	}
	if !isExistingDir(sourceRoot) {
		result.Reason = "module_root_missing"
		return result, nil
	}

	uploadRoot := strings.TrimSpace(h.uploadDir)
	if uploadRoot == "" {
		uploadRoot = filepath.Join("uploads", "plugins")
	}
	uploadRootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(uploadRoot)))
	if err != nil {
		return result, fmt.Errorf("resolve artifact root failed: %w", err)
	}
	targetRoot := pluginDataLayerRoot(uploadRootAbs, current.ID)
	result.TargetRoot = filepath.ToSlash(targetRoot)
	if targetRoot == "" {
		result.Reason = "data_root_empty"
		return result, nil
	}
	if !isPathWithinRoot(uploadRootAbs, sourceRoot) {
		result.Reason = "module_root_outside_artifact_root"
		return result, nil
	}
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return result, fmt.Errorf("prepare plugin data root failed: %w", err)
	}

	summary, err := copyMigratableJSWorkerFiles(sourceRoot, targetRoot)
	if err != nil {
		return result, err
	}
	result.Applied = true
	result.Reason = "ok"
	result.FilesCopied = summary.FilesCopied
	result.FilesSkipped = summary.FilesSkipped
	result.BytesCopied = summary.BytesCopied
	return result, nil
}

type jsWorkerFileCopySummary struct {
	FilesCopied  int
	FilesSkipped int
	BytesCopied  int64
}

func copyMigratableJSWorkerFiles(sourceRoot, targetRoot string) (jsWorkerFileCopySummary, error) {
	summary := jsWorkerFileCopySummary{}
	sourceRoot = filepath.Clean(sourceRoot)
	targetRoot = filepath.Clean(targetRoot)
	err := filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cleanPath := filepath.Clean(path)
		if cleanPath == sourceRoot {
			return nil
		}
		if !isPathWithinRoot(sourceRoot, cleanPath) {
			return fmt.Errorf("source path escapes module root: %s", cleanPath)
		}

		rel, err := filepath.Rel(sourceRoot, cleanPath)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" {
			return nil
		}

		targetPath := filepath.Clean(filepath.Join(targetRoot, rel))
		if !isPathWithinRoot(targetRoot, targetPath) {
			return fmt.Errorf("target path escapes module root: %s", targetPath)
		}

		if d.Type()&os.ModeSymlink != 0 {
			summary.FilesSkipped++
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if !shouldMigrateJSWorkerFile(rel) {
			summary.FilesSkipped++
			return nil
		}
		if info, err := os.Stat(targetPath); err == nil {
			if info.IsDir() {
				return fmt.Errorf("target path is directory while source is file: %s", targetPath)
			}
			// Preserve data-layer runtime state; activation migration is best-effort import.
			summary.FilesSkipped++
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		written, err := copyFileWithMode(cleanPath, targetPath, info.Mode().Perm())
		if err != nil {
			return err
		}
		summary.FilesCopied++
		summary.BytesCopied += written
		return nil
	})
	if err != nil {
		return summary, err
	}
	return summary, nil
}

func shouldMigrateJSWorkerFile(relPath string) bool {
	slashRel := filepath.ToSlash(filepath.Clean(relPath))
	loweredRel := strings.ToLower(slashRel)
	base := strings.ToLower(filepath.Base(loweredRel))

	switch base {
	case "manifest.json", "plugin.json", "plugin-manifest.json":
		return false
	}

	switch strings.ToLower(filepath.Ext(base)) {
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".map":
		return false
	}
	return true
}

func copyFileWithMode(sourcePath, targetPath string, mode os.FileMode) (int64, error) {
	src, err := os.Open(sourcePath)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	if mode == 0 {
		mode = 0644
	}
	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		return written, err
	}
	return written, nil
}

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func resolveArtifactScriptPath(address string, packagePath string) string {
	scriptPath, err := service.ResolveJSWorkerScriptPath(address, packagePath)
	if err != nil {
		return ""
	}
	return scriptPath
}

func detectArtifactModuleRoot(entryScriptPath string) string {
	trimmed := strings.TrimSpace(entryScriptPath)
	if trimmed == "" {
		return ""
	}
	entryDir := filepath.Clean(filepath.Dir(filepath.FromSlash(trimmed)))
	current := entryDir
	for i := 0; i < 16; i++ {
		if hasPluginManifest(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return entryDir
}

func hasPluginManifest(dir string) bool {
	manifestNames := []string{"manifest.json", "plugin.json", "plugin-manifest.json"}
	for _, name := range manifestNames {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func normalizeConfigJSON(formConfig string, manifest *pluginPackageManifest) (string, error) {
	if strings.TrimSpace(formConfig) != "" {
		return normalizeJSONObjectString(formConfig, "{}")
	}

	if manifest != nil && manifest.Config != nil {
		out, err := json.Marshal(manifest.Config)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}

	return "{}", nil
}

func normalizeRuntimeParamsJSON(formRuntimeParams string, manifest *pluginPackageManifest) (string, error) {
	if strings.TrimSpace(formRuntimeParams) != "" {
		return normalizeJSONObjectString(formRuntimeParams, "{}")
	}
	if manifest != nil && manifest.RuntimeParams != nil {
		out, err := json.Marshal(manifest.RuntimeParams)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return "{}", nil
}

func normalizeCapabilitiesJSON(formCapabilities string, fallback string) (string, error) {
	defaultValue := strings.TrimSpace(fallback)
	if defaultValue == "" {
		defaultValue = "{}"
	}
	if strings.TrimSpace(formCapabilities) != "" {
		normalized, err := normalizeJSONObjectString(formCapabilities, defaultValue)
		if err != nil {
			return "", err
		}
		return normalizeCapabilityPermissionGovernanceJSON(normalized)
	}
	normalized, err := normalizeJSONObjectString(defaultValue, "{}")
	if err != nil {
		return "", err
	}
	return normalizeCapabilityPermissionGovernanceJSON(normalized)
}

func normalizeCapabilityPermissionGovernanceJSON(raw string) (string, error) {
	capabilities := parseJSONObjectString(raw)
	if capabilities == nil {
		capabilities = map[string]interface{}{}
	}

	requests := collectCapabilityPermissionRequests(capabilities)
	requested := make([]string, 0, len(requests))
	for _, request := range requests {
		requested = append(requested, request.Key)
	}

	hasGrantedPermissions := false
	if _, exists := capabilities["granted_permissions"]; exists {
		hasGrantedPermissions = true
	}
	granted := service.NormalizePluginPermissionList(parseStringListFromAny(capabilities["granted_permissions"]))
	if len(requests) > 0 {
		if !hasGrantedPermissions {
			granted = service.DefaultGrantedPluginPermissions(requests)
		}
		validatedGranted, err := service.ValidateGrantedPluginPermissions(requests, granted)
		if err != nil {
			return "", err
		}
		granted = validatedGranted
	}

	capabilities["requested_permissions"] = service.NormalizePluginPermissionList(requested)
	capabilities["granted_permissions"] = granted

	if required := normalizeCapabilityRequiredPermissions(capabilities); len(required) > 0 {
		capabilities["required_permissions"] = required
	} else {
		delete(capabilities, "required_permissions")
	}

	body, err := json.Marshal(capabilities)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func collectCapabilityPermissionRequests(capabilities map[string]interface{}) []service.PluginPermissionRequest {
	if len(capabilities) == 0 {
		return nil
	}

	requested := parseStringListFromAny(capabilities["requested_permissions"])
	required := normalizeCapabilityRequiredPermissions(capabilities)
	reasonByKey := make(map[string]string)

	for _, item := range parseCapabilityPermissionDescriptors(capabilities["permissions"]) {
		key := service.NormalizePluginPermissionKey(item.Key)
		if key == "" {
			continue
		}
		requested = append(requested, key)
		if item.Required {
			required = append(required, key)
		}
		if reason := item.Reason.String(); reason != "" {
			reasonByKey[key] = reason
		}
	}

	htmlMode := normalizePluginFrontendHTMLMode(parseStringFromAny(
		capabilities["frontend_html_mode"],
		capabilities["html_mode"],
	))
	if htmlMode == pluginFrontendHTMLModeTrusted {
		requested = append(requested, service.PluginPermissionFrontendHTMLTrust)
		if strings.TrimSpace(reasonByKey[service.PluginPermissionFrontendHTMLTrust]) == "" {
			reasonByKey[service.PluginPermissionFrontendHTMLTrust] = "Plugin requests trusted HTML rendering without sanitize fallback."
		}
	}

	return service.BuildPluginPermissionRequests(requested, required, reasonByKey)
}

func normalizeCapabilityRequiredPermissions(capabilities map[string]interface{}) []string {
	if len(capabilities) == 0 {
		return nil
	}

	required := parseStringListFromAny(capabilities["required_permissions"])
	for _, item := range parseCapabilityPermissionDescriptors(capabilities["permissions"]) {
		if !item.Required {
			continue
		}
		key := service.NormalizePluginPermissionKey(item.Key)
		if key == "" {
			continue
		}
		required = append(required, key)
	}
	return service.NormalizePluginPermissionList(required)
}

func parseCapabilityPermissionDescriptors(raw interface{}) []pluginPackagePermission {
	if raw == nil {
		return nil
	}

	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var items []pluginPackagePermission
	if err := json.Unmarshal(body, &items); err != nil {
		return nil
	}
	return items
}

func manifestCapabilitiesJSON(manifest *pluginPackageManifest) string {
	if manifest == nil || manifest.Capabilities == nil {
		return ""
	}
	out, err := json.Marshal(manifest.Capabilities)
	if err != nil {
		return ""
	}
	return string(out)
}

func collectManifestPermissionRequests(manifest *pluginPackageManifest) []service.PluginPermissionRequest {
	if manifest == nil {
		return nil
	}

	requested := make([]string, 0)
	required := make([]string, 0)
	reasonByKey := make(map[string]string)

	requested = append(requested, manifest.RequestedPermissions...)
	required = append(required, manifest.RequiredPermissions...)
	for _, item := range manifest.Permissions {
		key := service.NormalizePluginPermissionKey(item.Key)
		if key == "" {
			continue
		}
		requested = append(requested, key)
		if item.Required {
			required = append(required, key)
		}
		reasonByKey[key] = item.Reason.String()
	}

	if manifest.Capabilities != nil {
		htmlMode := normalizePluginFrontendHTMLMode(parseStringFromAny(
			manifest.Capabilities["frontend_html_mode"],
			manifest.Capabilities["html_mode"],
		))
		if htmlMode == pluginFrontendHTMLModeTrusted {
			requested = append(requested, service.PluginPermissionFrontendHTMLTrust)
			if strings.TrimSpace(reasonByKey[service.PluginPermissionFrontendHTMLTrust]) == "" {
				reasonByKey[service.PluginPermissionFrontendHTMLTrust] = "Plugin requests trusted HTML rendering without sanitize fallback."
			}
		}

		var caps struct {
			RequestedPermissions []string                  `json:"requested_permissions"`
			RequiredPermissions  []string                  `json:"required_permissions"`
			Permissions          []pluginPackagePermission `json:"permissions"`
		}
		raw, err := json.Marshal(manifest.Capabilities)
		if err == nil {
			if err := json.Unmarshal(raw, &caps); err == nil {
				requested = append(requested, caps.RequestedPermissions...)
				required = append(required, caps.RequiredPermissions...)
				for _, item := range caps.Permissions {
					key := service.NormalizePluginPermissionKey(item.Key)
					if key == "" {
						continue
					}
					requested = append(requested, key)
					if item.Required {
						required = append(required, key)
					}
					if item.Reason.String() != "" {
						reasonByKey[key] = item.Reason.String()
					}
				}
			}
		}
	}

	return service.BuildPluginPermissionRequests(requested, required, reasonByKey)
}

func collectManifestGrantedPermissions(manifest *pluginPackageManifest) []string {
	if manifest == nil || len(manifest.Capabilities) == 0 {
		return nil
	}

	var caps struct {
		GrantedPermissions []string `json:"granted_permissions"`
	}
	raw, err := json.Marshal(manifest.Capabilities)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(raw, &caps); err != nil {
		return nil
	}
	return service.NormalizePluginPermissionList(caps.GrantedPermissions)
}

func collectManifestDefaultGrantedPermissions(
	manifest *pluginPackageManifest,
	requests []service.PluginPermissionRequest,
) []string {
	defaultGranted := service.DefaultGrantedPluginPermissions(requests)
	manifestGranted := collectManifestGrantedPermissions(manifest)
	if len(manifestGranted) == 0 {
		return defaultGranted
	}

	merged := service.NormalizePluginPermissionList(append(defaultGranted, manifestGranted...))
	validated, err := service.ValidateGrantedPluginPermissions(requests, merged)
	if err != nil {
		return merged
	}
	return validated
}

func resolveGrantedPermissions(raw string, requests []service.PluginPermissionRequest, defaultGranted []string) ([]string, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		resolvedDefault := service.NormalizePluginPermissionList(defaultGranted)
		if len(resolvedDefault) == 0 {
			resolvedDefault = service.DefaultGrantedPluginPermissions(requests)
		}
		return service.ValidateGrantedPluginPermissions(requests, resolvedDefault)
	}

	granted := make([]string, 0)
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &granted); err != nil {
			return nil, fmt.Errorf("granted_permissions must be a JSON string array")
		}
	} else {
		parts := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})
		granted = append(granted, parts...)
	}

	return service.ValidateGrantedPluginPermissions(requests, granted)
}

func applyPermissionGrantToCapabilities(capabilitiesJSON string, requests []service.PluginPermissionRequest, granted []string) (string, error) {
	capabilities := map[string]interface{}{}
	trimmed := strings.TrimSpace(capabilitiesJSON)
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &capabilities); err != nil {
			return "", err
		}
	}

	requested := make([]string, 0, len(requests))
	for _, request := range requests {
		requested = append(requested, request.Key)
	}
	requested = service.NormalizePluginPermissionList(requested)
	granted = service.NormalizePluginPermissionList(granted)

	capabilities["requested_permissions"] = requested
	capabilities["granted_permissions"] = granted

	body, err := json.Marshal(capabilities)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func normalizeJSONObjectString(raw string, defaultValue string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if strings.TrimSpace(defaultValue) == "" {
			return "{}", nil
		}
		trimmed = strings.TrimSpace(defaultValue)
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", err
	}
	if decoded == nil {
		decoded = map[string]interface{}{}
	}
	if _, ok := decoded.(map[string]interface{}); !ok {
		return "", fmt.Errorf("json must be object")
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseJSONObjectString(raw string) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	return decoded
}

func asStringAnyMap(value interface{}) map[string]interface{} {
	typed, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return typed
}

func mapValue(source map[string]interface{}, key string) interface{} {
	if len(source) == 0 {
		return nil
	}
	return source[key]
}

func normalizeManifestPluginPagePath(value interface{}, area string) string {
	path := strings.TrimSpace(parseStringFromAny(value))
	if path == "" {
		return ""
	}
	normalized := normalizeFrontendBootstrapPath(path)
	if !isAllowedPluginPagePath(area, normalized) {
		return ""
	}
	return normalized
}

func extractPluginManifestPagePathsFromMap(manifest map[string]interface{}) (string, string) {
	if len(manifest) == 0 {
		return "", ""
	}

	frontend := asStringAnyMap(manifest["frontend"])
	adminPage := asStringAnyMap(mapValue(frontend, "admin_page"))
	userPage := asStringAnyMap(mapValue(frontend, "user_page"))
	if adminPage == nil {
		adminPage = asStringAnyMap(manifest["admin_page"])
	}
	if userPage == nil {
		userPage = asStringAnyMap(manifest["user_page"])
	}

	adminPath := normalizeManifestPluginPagePath(
		parseStringFromAny(
			mapValue(adminPage, "path"),
			mapValue(frontend, "admin_page_path"),
			manifest["admin_page_path"],
		),
		frontendBootstrapAreaAdmin,
	)
	userPath := normalizeManifestPluginPagePath(
		parseStringFromAny(
			mapValue(userPage, "path"),
			mapValue(frontend, "user_page_path"),
			manifest["user_page_path"],
		),
		frontendBootstrapAreaUser,
	)
	return adminPath, userPath
}

func extractPluginManifestPagePaths(rawManifest string) (string, string) {
	return extractPluginManifestPagePathsFromMap(parseJSONObjectString(rawManifest))
}

func validatePluginPackageManifest(manifest *pluginPackageManifest) error {
	if manifest == nil {
		return nil
	}
	if err := validatePluginPackageSchema(manifest.ConfigSchema, "config_schema"); err != nil {
		return err
	}
	if err := validatePluginPackageSchema(manifest.SecretSchema, "secret_schema"); err != nil {
		return err
	}
	if err := validatePluginPackageSchema(manifest.RuntimeParamsSchema, "runtime_params_schema"); err != nil {
		return err
	}
	if err := validatePluginPackageFrontendManifest(manifest.Frontend); err != nil {
		return err
	}
	if err := validatePluginPackageWorkspaceManifest(manifest.Workspace); err != nil {
		return err
	}
	if err := validatePluginPackageWebhookManifests(manifest.Webhooks); err != nil {
		return err
	}
	if err := validatePluginPackageCompatibility(manifest, manifest.Runtime); err != nil {
		return err
	}
	return nil
}

func validatePluginPackageWorkspaceManifest(workspace *pluginPackageWorkspaceManifest) error {
	if workspace == nil {
		return nil
	}
	// Workspace command discovery is runtime-driven now. Keep accepting the
	// legacy workspace manifest shape for compatibility, but do not enforce
	// command-level schema here because the host no longer consumes it.
	return nil
}

func validatePluginPackageCompatibility(manifest *pluginPackageManifest, runtime string) error {
	if manifest == nil {
		return nil
	}
	inspection := service.InspectPluginManifestCompatibilityMetadata(
		runtime,
		manifest.ManifestVersion,
		manifest.ProtocolVersion,
		manifest.MinHostProtocolVersion,
		manifest.MaxHostProtocolVersion,
		true,
	)
	if inspection.Compatible {
		return nil
	}
	return pluginManifestSchemaValidationError(
		pluginManifestCompatibilityFieldPath(inspection.ReasonCode),
		"%s",
		inspection.Reason,
	)
}

func pluginManifestCompatibilityFieldPath(reasonCode string) string {
	switch strings.ToLower(strings.TrimSpace(reasonCode)) {
	case "invalid_manifest_version", "manifest_version_unsupported":
		return "manifest_version"
	case "invalid_protocol_version", "protocol_version_unsupported":
		return "protocol_version"
	case "invalid_min_host_protocol_version", "host_protocol_too_old":
		return "min_host_protocol_version"
	case "invalid_max_host_protocol_version", "host_protocol_too_new":
		return "max_host_protocol_version"
	case "invalid_host_protocol_range":
		return "min_host_protocol_version"
	default:
		return "manifest"
	}
}

func validatePluginPackageFrontendManifest(frontend *pluginPackageFrontendManifest) error {
	if frontend == nil {
		return nil
	}
	if err := validatePluginPackageFrontendPage(frontend.AdminPage, frontendBootstrapAreaAdmin, "frontend.admin_page"); err != nil {
		return err
	}
	if err := validatePluginPackageFrontendPage(frontend.UserPage, frontendBootstrapAreaUser, "frontend.user_page"); err != nil {
		return err
	}
	return nil
}

func validatePluginPackageFrontendPage(page *pluginPackageFrontendPage, area string, fieldPath string) error {
	if page == nil {
		return nil
	}
	if strings.TrimSpace(page.Path) == "" {
		return pluginManifestSchemaValidationError(fieldPath+".path", "is required")
	}

	normalizedPath := normalizeFrontendBootstrapPath(page.Path)
	if !isAllowedPluginPagePath(area, normalizedPath) {
		expectedPrefix := "/plugin-pages/"
		if area == frontendBootstrapAreaAdmin {
			expectedPrefix = "/admin/plugin-pages/"
		}
		return pluginManifestSchemaValidationError(fieldPath+".path", "must start with %q", expectedPrefix)
	}
	page.Path = normalizedPath
	return nil
}

func newPluginManifestJSONBizError(cause string) error {
	params := map[string]interface{}{}
	if strings.TrimSpace(cause) != "" {
		params["cause"] = strings.TrimSpace(cause)
	}
	return bizerr.New(
		"plugin.admin.http_400.invalid_package_manifest_json",
		"invalid package manifest json",
	).WithParams(params)
}

func newPluginManifestSchemaBizError(path string, reason string) error {
	params := map[string]interface{}{
		"path":   strings.TrimSpace(path),
		"reason": strings.TrimSpace(reason),
	}
	return bizerr.New(
		"plugin.admin.http_400.invalid_package_manifest_schema",
		"invalid package manifest schema",
	).WithParams(params)
}

func pluginManifestSchemaValidationError(path string, format string, args ...interface{}) error {
	return newPluginManifestSchemaBizError(path, fmt.Sprintf(format, args...))
}

func validatePluginPackageSchema(schema map[string]interface{}, schemaName string) error {
	if schema == nil {
		return nil
	}
	if err := validatePluginSchemaOptionalString(schema, "title", schemaName); err != nil {
		return err
	}
	if err := validatePluginSchemaOptionalString(schema, "description", schemaName); err != nil {
		return err
	}

	fieldsRaw, exists := schema["fields"]
	if !exists {
		return pluginManifestSchemaValidationError(schemaName+".fields", "is required")
	}
	fields, ok := fieldsRaw.([]interface{})
	if !ok || len(fields) == 0 {
		return pluginManifestSchemaValidationError(schemaName+".fields", "must be a non-empty array")
	}

	seenKeys := make(map[string]struct{}, len(fields))
	for idx, item := range fields {
		fieldPath := fmt.Sprintf("%s.fields[%d]", schemaName, idx)
		field, ok := item.(map[string]interface{})
		if !ok {
			return pluginManifestSchemaValidationError(fieldPath, "must be an object")
		}

		keyValue, ok := field["key"].(string)
		if !ok || strings.TrimSpace(keyValue) == "" {
			return pluginManifestSchemaValidationError(fieldPath+".key", "is required and must be a non-empty string")
		}
		key := strings.TrimSpace(keyValue)
		if _, exists := seenKeys[key]; exists {
			return pluginManifestSchemaValidationError(fieldPath+".key", "duplicates %q", key)
		}
		seenKeys[key] = struct{}{}

		if err := validatePluginSchemaOptionalString(field, "label", fieldPath); err != nil {
			return err
		}
		if err := validatePluginSchemaOptionalString(field, "description", fieldPath); err != nil {
			return err
		}
		if err := validatePluginSchemaOptionalString(field, "placeholder", fieldPath); err != nil {
			return err
		}
		if err := validatePluginSchemaOptionalBool(field, "required", fieldPath); err != nil {
			return err
		}

		fieldType, err := normalizePluginPackageSchemaFieldType(field["type"])
		if err != nil {
			return pluginManifestSchemaValidationError(fieldPath+".type", "value %v is unsupported", field["type"])
		}
		if err := validatePluginPackageSchemaDefaultValue(field["default"], fieldType, fieldPath); err != nil {
			return err
		}

		options, hasOptions, err := validatePluginPackageSchemaFieldOptions(field["options"], fieldPath)
		if err != nil {
			return err
		}
		if fieldType == "select" {
			if !hasOptions || len(options) == 0 {
				return pluginManifestSchemaValidationError(fieldPath+".options", "must be a non-empty array for select fields")
			}
			if defaultValue, exists := field["default"]; exists {
				defaultKey := pluginSchemaValueFingerprint(defaultValue)
				matched := false
				for _, optionKey := range options {
					if optionKey == defaultKey {
						matched = true
						break
					}
				}
				if !matched {
					return pluginManifestSchemaValidationError(fieldPath+".default", "must match one of the select options")
				}
			}
		}
	}
	return nil
}

func validatePluginSchemaOptionalString(source map[string]interface{}, key string, path string) error {
	value, exists := source[key]
	if !exists || value == nil {
		return nil
	}
	if _, ok := value.(string); !ok {
		return pluginManifestSchemaValidationError(path+"."+key, "must be a string")
	}
	return nil
}

func validatePluginSchemaOptionalBool(source map[string]interface{}, key string, path string) error {
	value, exists := source[key]
	if !exists || value == nil {
		return nil
	}
	if _, ok := value.(bool); !ok {
		return pluginManifestSchemaValidationError(path+"."+key, "must be a boolean")
	}
	return nil
}

func normalizePluginPackageSchemaFieldType(raw interface{}) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if raw == nil || trimmed == "" || trimmed == "<nil>" {
		return "string", nil
	}
	switch trimmed {
	case "string", "textarea", "number", "boolean", "select", "json", "secret":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported field type %q", trimmed)
	}
}

func validatePluginPackageSchemaDefaultValue(value interface{}, fieldType string, fieldPath string) error {
	if value == nil {
		return nil
	}
	switch fieldType {
	case "string", "textarea", "secret":
		if _, ok := value.(string); !ok {
			return pluginManifestSchemaValidationError(fieldPath+".default", "must be a string")
		}
	case "number":
		if !isPluginSchemaNumber(value) {
			return pluginManifestSchemaValidationError(fieldPath+".default", "must be a number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return pluginManifestSchemaValidationError(fieldPath+".default", "must be a boolean")
		}
	case "select", "json":
		return nil
	}
	return nil
}

func validatePluginPackageWebhookManifests(items []pluginPackageWebhookManifest) error {
	if len(items) == 0 {
		return nil
	}

	seenKeys := make(map[string]struct{}, len(items))
	for idx := range items {
		fieldPath := fmt.Sprintf("webhooks[%d]", idx)
		key := strings.TrimSpace(items[idx].Key)
		if key == "" {
			return pluginManifestSchemaValidationError(fieldPath+".key", "is required")
		}
		if _, exists := seenKeys[key]; exists {
			return pluginManifestSchemaValidationError(fieldPath+".key", "duplicates %q", key)
		}
		seenKeys[key] = struct{}{}

		if strings.TrimSpace(items[idx].Action) == "" {
			items[idx].Action = "webhook." + key
		}
		if _, err := normalizePluginWebhookMethod(items[idx].Method); err != nil {
			return pluginManifestSchemaValidationError(fieldPath+".method", "%s", err.Error())
		}
		authMode, err := normalizePluginWebhookAuthMode(items[idx].AuthMode)
		if err != nil {
			return pluginManifestSchemaValidationError(fieldPath+".auth_mode", "%s", err.Error())
		}
		if authMode != "none" && strings.TrimSpace(items[idx].SecretKey) == "" {
			return pluginManifestSchemaValidationError(fieldPath+".secret_key", "is required when auth_mode is %q", authMode)
		}
	}
	return nil
}

func normalizePluginWebhookMethod(raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if method == "" {
		return http.MethodPost, nil
	}
	if method == "*" || method == "ANY" {
		return "*", nil
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return method, nil
	default:
		return "", fmt.Errorf("must be one of GET/POST/PUT/PATCH/DELETE/*")
	}
}

func normalizePluginWebhookAuthMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return "none", nil
	}
	switch mode {
	case "none", "query", "header", "hmac_sha256":
		return mode, nil
	default:
		return "", fmt.Errorf("must be one of none/query/header/hmac_sha256")
	}
}

func validatePluginPackageSchemaFieldOptions(raw interface{}, fieldPath string) ([]string, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, true, pluginManifestSchemaValidationError(fieldPath+".options", "must be an array")
	}

	fingerprints := make([]string, 0, len(items))
	for idx, item := range items {
		optionPath := fmt.Sprintf("%s.options[%d]", fieldPath, idx)
		if item == nil {
			continue
		}
		switch typed := item.(type) {
		case map[string]interface{}:
			if err := validatePluginSchemaOptionalString(typed, "label", optionPath); err != nil {
				return nil, true, err
			}
			if err := validatePluginSchemaOptionalString(typed, "description", optionPath); err != nil {
				return nil, true, err
			}
			value, hasValue := typed["value"]
			if !hasValue {
				keyValue, ok := typed["key"].(string)
				if !ok || strings.TrimSpace(keyValue) == "" {
					return nil, true, pluginManifestSchemaValidationError(optionPath+".value", "is required when key is absent")
				}
				value = keyValue
			}
			fingerprints = append(fingerprints, pluginSchemaValueFingerprint(value))
		case string, float64, bool:
			fingerprints = append(fingerprints, pluginSchemaValueFingerprint(typed))
		default:
			return nil, true, pluginManifestSchemaValidationError(optionPath, "must be a scalar or object")
		}
	}
	return fingerprints, true, nil
}

func isPluginSchemaNumber(value interface{}) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func pluginSchemaValueFingerprint(value interface{}) string {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(body)
}

func readManifestFromPackage(packagePath string) (*pluginPackageManifest, string, error) {
	if strings.ToLower(filepath.Ext(packagePath)) != ".zip" {
		return nil, "", nil
	}

	reader, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read package zip: %w", err)
	}
	defer reader.Close()

	manifestNames := map[string]bool{
		"manifest.json":        true,
		"plugin.json":          true,
		"plugin-manifest.json": true,
	}

	for _, file := range reader.File {
		base := strings.ToLower(filepath.Base(file.Name))
		if !manifestNames[base] {
			continue
		}
		rc, openErr := file.Open()
		if openErr != nil {
			return nil, "", fmt.Errorf("failed to open manifest from package: %w", openErr)
		}
		raw, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("failed to read manifest from package: %w", readErr)
		}

		var manifest pluginPackageManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, "", newPluginManifestJSONBizError(err.Error())
		}
		if err := validatePluginPackageManifest(&manifest); err != nil {
			return nil, "", err
		}
		return &manifest, string(raw), nil
	}

	return nil, "", nil
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "package.bin"
	}

	var builder strings.Builder
	for _, r := range base {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum || r == '.' || r == '_' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	result := strings.TrimSpace(builder.String())
	if result == "" {
		return "package.bin"
	}
	return result
}

func parseBoolForm(value string, defaultValue bool) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return defaultValue
	}

	switch trimmed {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed != "" {
		return trimmed
	}
	return "0.0.0"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeLowerStringListValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func manifestField(manifest *pluginPackageManifest, field string) string {
	if manifest == nil {
		return ""
	}
	switch field {
	case "name":
		return manifest.Name
	case "display_name":
		return manifest.DisplayName.String()
	case "description":
		return manifest.Description.String()
	case "type":
		return manifest.Type
	case "runtime":
		return manifest.Runtime
	case "address":
		return manifest.Address
	case "entry":
		return manifest.Entry
	case "version":
		return manifest.Version
	case "changelog":
		return manifest.Changelog.String()
	default:
		return ""
	}
}

func manifestBoolField(manifest *pluginPackageManifest, field string, fallback bool) bool {
	if manifest == nil {
		return fallback
	}
	switch field {
	case "activate":
		if manifest.Activate != nil {
			return *manifest.Activate
		}
	case "auto_start":
		if manifest.AutoStart != nil {
			return *manifest.AutoStart
		}
	}
	return fallback
}
