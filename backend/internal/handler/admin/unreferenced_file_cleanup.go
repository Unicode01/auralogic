package admin

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

type unreferencedFileCleanupStats struct {
	DeletedFiles int64 `json:"deleted_files"`
	DeletedDirs  int64 `json:"deleted_dirs"`
	DeletedBytes int64 `json:"deleted_bytes"`
}

type filesystemCleanupStats struct {
	DeletedFiles int64
	DeletedDirs  int64
	DeletedBytes int64
}

type cleanupCandidate struct {
	Path string
	Size int64
}

func (h *SettingsHandler) cleanupUnreferencedFiles() (unreferencedFileCleanupStats, error) {
	stats := unreferencedFileCleanupStats{}

	uploadDir := strings.TrimSpace(h.cfg.Upload.Dir)
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	uploadRoot, err := filepath.Abs(filepath.Clean(filepath.FromSlash(uploadDir)))
	if err != nil {
		return stats, err
	}

	productRoot := filepath.Clean(filepath.Join(uploadRoot, "products"))
	productRefs, err := collectProductUploadReferences(h.db, h.cfg, productRoot)
	if err != nil {
		return stats, err
	}
	productCleanup, err := cleanupUnreferencedFilesInRoot(productRoot, productRefs, nil)
	if err != nil {
		return stats, err
	}
	stats.DeletedFiles += productCleanup.DeletedFiles
	stats.DeletedDirs += productCleanup.DeletedDirs
	stats.DeletedBytes += productCleanup.DeletedBytes

	ticketRoot := filepath.Clean(filepath.Join(uploadRoot, "tickets"))
	ticketRefs, err := collectTicketUploadReferences(h.db, ticketRoot)
	if err != nil {
		return stats, err
	}
	ticketCleanup, err := cleanupUnreferencedFilesInRoot(ticketRoot, ticketRefs, nil)
	if err != nil {
		return stats, err
	}
	stats.DeletedFiles += ticketCleanup.DeletedFiles
	stats.DeletedDirs += ticketCleanup.DeletedDirs
	stats.DeletedBytes += ticketCleanup.DeletedBytes

	artifactRoot := strings.TrimSpace(h.cfg.Plugin.ArtifactDir)
	if artifactRoot == "" {
		artifactRoot = filepath.Join("data", "plugins")
	}
	artifactRootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(artifactRoot)))
	if err != nil {
		return stats, err
	}
	pluginFileRefs, pluginDirRefs, err := collectPluginArtifactReferences(h.db, artifactRootAbs)
	if err != nil {
		return stats, err
	}
	pluginCleanup, err := cleanupUnreferencedFilesInRoot(artifactRootAbs, pluginFileRefs, pluginDirRefs)
	if err != nil {
		return stats, err
	}
	stats.DeletedFiles += pluginCleanup.DeletedFiles
	stats.DeletedDirs += pluginCleanup.DeletedDirs
	stats.DeletedBytes += pluginCleanup.DeletedBytes

	return stats, nil
}

func cleanupUnreferencedFilesInRoot(root string, referencedFiles map[string]struct{}, referencedDirRoots []string) (filesystemCleanupStats, error) {
	stats := filesystemCleanupStats{}
	if strings.TrimSpace(root) == "" {
		return stats, nil
	}

	rootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(root)))
	if err != nil {
		return stats, err
	}
	if _, err := os.Stat(rootAbs); os.IsNotExist(err) {
		return stats, nil
	} else if err != nil {
		return stats, err
	}

	referencedDirs := make([]string, 0, len(referencedDirRoots))
	referencedDirSet := make(map[string]struct{}, len(referencedDirRoots))
	for _, dir := range referencedDirRoots {
		normalized := strings.TrimSpace(dir)
		if normalized == "" {
			continue
		}
		abs, absErr := filepath.Abs(filepath.Clean(filepath.FromSlash(normalized)))
		if absErr != nil {
			continue
		}
		if abs == rootAbs || !isPathWithinRoot(rootAbs, abs) {
			continue
		}
		referencedDirs = append(referencedDirs, abs)
		referencedDirSet[abs] = struct{}{}
	}
	sort.Slice(referencedDirs, func(i, j int) bool {
		return len(referencedDirs[i]) > len(referencedDirs[j])
	})

	dirs := make([]string, 0)
	candidates := make([]cleanupCandidate, 0)
	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		cleanPath := filepath.Clean(path)
		if cleanPath == rootAbs {
			return nil
		}

		if d.IsDir() {
			dirs = append(dirs, cleanPath)
			if _, ok := referencedDirSet[cleanPath]; ok {
				return filepath.SkipDir
			}
			return nil
		}

		if _, ok := referencedFiles[cleanPath]; ok {
			return nil
		}
		if isPathWithinAny(cleanPath, referencedDirs) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		candidates = append(candidates, cleanupCandidate{
			Path: cleanPath,
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return stats, err
	}

	for _, candidate := range candidates {
		if err := os.Remove(candidate.Path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return stats, err
		}
		stats.DeletedFiles++
		stats.DeletedBytes += candidate.Size
	}

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, dir := range dirs {
		if _, ok := referencedDirSet[dir]; ok {
			continue
		}
		if err := os.Remove(dir); err == nil {
			stats.DeletedDirs++
			continue
		}
		if os.IsNotExist(err) {
			continue
		}
	}

	return stats, nil
}

func collectProductUploadReferences(db *gorm.DB, cfg *config.Config, productRoot string) (map[string]struct{}, error) {
	references := make(map[string]struct{})
	addUploadReferenceFromText(references, productRoot, "products", cfg.Customization.LogoURL)
	addUploadReferenceFromText(references, productRoot, "products", cfg.Customization.FaviconURL)
	addUploadReferenceFromText(references, productRoot, "products", cfg.Customization.AuthBranding.CustomHTML)
	for _, rule := range cfg.Customization.PageRules {
		addUploadReferenceFromText(references, productRoot, "products", rule.CSS)
		addUploadReferenceFromText(references, productRoot, "products", rule.JS)
	}
	if err := collectUploadReferencesFromTemplateFiles(references, productRoot, "products", filepath.Join("templates", "email")); err != nil {
		return nil, err
	}

	var products []models.Product
	if err := db.Unscoped().Select("images", "description", "short_description").Find(&products).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, product := range products {
		for _, image := range product.Images {
			addUploadReferenceFromText(references, productRoot, "products", image.URL)
		}
		addUploadReferenceFromText(references, productRoot, "products", product.Description)
		addUploadReferenceFromText(references, productRoot, "products", product.ShortDescription)
	}

	var orders []models.Order
	if err := db.Unscoped().Select("items").Find(&orders).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, order := range orders {
		for _, item := range order.Items {
			addUploadReferenceFromText(references, productRoot, "products", item.ImageURL)
		}
	}

	var cartItems []models.CartItem
	if err := db.Unscoped().Select("image_url").Find(&cartItems).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, item := range cartItems {
		addUploadReferenceFromText(references, productRoot, "products", item.ImageURL)
	}

	var users []models.User
	if err := db.Unscoped().Select("avatar").Find(&users).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, user := range users {
		addUploadReferenceFromText(references, productRoot, "products", user.Avatar)
	}

	var articles []models.KnowledgeArticle
	if err := db.Unscoped().Select("content").Find(&articles).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, article := range articles {
		addUploadReferenceFromText(references, productRoot, "products", article.Content)
	}

	var announcements []models.Announcement
	if err := db.Unscoped().Select("content").Find(&announcements).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, announcement := range announcements {
		addUploadReferenceFromText(references, productRoot, "products", announcement.Content)
	}

	var pages []models.LandingPage
	if err := db.Unscoped().Select("html_content").Find(&pages).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, page := range pages {
		addUploadReferenceFromText(references, productRoot, "products", page.HTMLContent)
	}

	var marketingBatches []models.MarketingBatch
	if err := db.Unscoped().Select("content").Find(&marketingBatches).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, batch := range marketingBatches {
		addUploadReferenceFromText(references, productRoot, "products", batch.Content)
	}

	var templateVersions []models.TemplateVersion
	if err := db.Unscoped().Select("content_snapshot").Find(&templateVersions).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, version := range templateVersions {
		addUploadReferenceFromText(references, productRoot, "products", version.ContentSnapshot)
	}

	return references, nil
}

func collectTicketUploadReferences(db *gorm.DB, ticketRoot string) (map[string]struct{}, error) {
	references := make(map[string]struct{})

	var tickets []models.Ticket
	if err := db.Unscoped().Select("content").Find(&tickets).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, ticket := range tickets {
		addUploadReferenceFromText(references, ticketRoot, "tickets", ticket.Content)
	}

	var messages []models.TicketMessage
	if err := db.Unscoped().Select("content", "metadata").Find(&messages).Error; err != nil && !isMissingTableError(err) {
		return nil, err
	}
	for _, message := range messages {
		addUploadReferenceFromText(references, ticketRoot, "tickets", message.Content)
		addUploadReferenceFromText(references, ticketRoot, "tickets", string(message.Metadata))
	}

	return references, nil
}

func collectPluginArtifactReferences(db *gorm.DB, artifactRoot string) (map[string]struct{}, []string, error) {
	referencedFiles := make(map[string]struct{})
	referencedDirs := make(map[string]struct{})

	var plugins []models.Plugin
	if err := db.Find(&plugins).Error; err != nil && !isMissingTableError(err) {
		return nil, nil, err
	}
	for _, plugin := range plugins {
		addPluginPackageReference(referencedFiles, referencedDirs, artifactRoot, plugin.PackagePath)
		addArtifactAddressPath(referencedFiles, referencedDirs, artifactRoot, plugin.Address, plugin.PackagePath)
		if dataRoot := pluginDataLayerRoot(artifactRoot, plugin.ID); dataRoot != "" {
			referencedDirs[dataRoot] = struct{}{}
		}
	}

	var versions []models.PluginVersion
	if err := db.Find(&versions).Error; err != nil && !isMissingTableError(err) {
		return nil, nil, err
	}
	for _, version := range versions {
		addPluginPackageReference(referencedFiles, referencedDirs, artifactRoot, version.PackagePath)
		addArtifactAddressPath(referencedFiles, referencedDirs, artifactRoot, version.Address, version.PackagePath)
	}

	dirs := make([]string, 0, len(referencedDirs))
	for dir := range referencedDirs {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	return referencedFiles, dirs, nil
}

func addPluginPackageReference(fileRefs, dirRefs map[string]struct{}, artifactRoot string, packagePath string) {
	resolved, ok := normalizeArtifactPathWithinUploadRoot(artifactRoot, packagePath)
	if !ok {
		return
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		dirRefs[resolved] = struct{}{}
		return
	}
	fileRefs[resolved] = struct{}{}
}

func collectUploadReferencesFromTemplateFiles(references map[string]struct{}, root, area, dir string) error {
	dirAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(dir)))
	if err != nil {
		return err
	}
	if _, err := os.Stat(dirAbs); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return filepath.WalkDir(dirAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		addUploadReferenceFromText(references, root, area, string(content))
		return nil
	})
}

func addUploadReferenceFromText(references map[string]struct{}, root, area, text string) {
	for _, relativePath := range collectUploadRelativePaths(text, area) {
		resolved, ok := resolveUploadReferencePath(root, relativePath)
		if !ok {
			continue
		}
		references[resolved] = struct{}{}
	}
}

func collectUploadRelativePaths(text, area string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	markers := []string{
		"/uploads/" + area + "/",
		"uploads/" + area + "/",
	}
	seen := make(map[string]struct{})
	results := make([]string, 0)
	for _, marker := range markers {
		searchStart := 0
		for searchStart < len(trimmed) {
			index := strings.Index(trimmed[searchStart:], marker)
			if index < 0 {
				break
			}
			pathStart := searchStart + index + len(marker)
			pathEnd := pathStart
			for pathEnd < len(trimmed) {
				switch trimmed[pathEnd] {
				case ' ', '\t', '\r', '\n', '"', '\'', ')', '(', '<', '>', ']':
					goto endPath
				default:
					pathEnd++
				}
			}
		endPath:
			if pathEnd > pathStart {
				relativePath := trimmed[pathStart:pathEnd]
				relativePath = strings.SplitN(relativePath, "?", 2)[0]
				relativePath = strings.SplitN(relativePath, "#", 2)[0]
				if _, exists := seen[relativePath]; !exists {
					seen[relativePath] = struct{}{}
					results = append(results, relativePath)
				}
			}
			searchStart = pathEnd
		}
	}
	return results
}

func resolveUploadReferencePath(root, relativePath string) (string, bool) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", false
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "..") {
		return "", false
	}

	rootAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(root)))
	if err != nil {
		return "", false
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, cleaned))
	if err != nil {
		return "", false
	}
	targetAbs = filepath.Clean(targetAbs)
	if targetAbs == rootAbs || !isPathWithinRoot(rootAbs, targetAbs) {
		return "", false
	}
	return targetAbs, true
}

func isPathWithinAny(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || isPathWithinRoot(root, path) {
			return true
		}
	}
	return false
}

func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}
