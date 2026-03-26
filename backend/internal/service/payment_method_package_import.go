package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

type paymentMethodPackageImportOptions struct {
	Coordinates   pluginHostMarketCoordinates
	Source        map[string]interface{}
	Release       map[string]interface{}
	Governance    map[string]interface{}
	Download      map[string]interface{}
	Compatibility map[string]interface{}
	Warnings      []string
}

type generatedPaymentMethodPackageDefinition struct {
	ArtifactName  string
	DisplayName   string
	Description   string
	Icon          string
	Version       string
	PollInterval  int
	Config        map[string]interface{}
	Script        string
	PackageName   string
	EntryFileName string
}

type builtinPaymentPackageDefinition = generatedPaymentMethodPackageDefinition

type legacyPaymentMethodImportInput struct {
	TargetMethodID uint
	Name           string
	Description    string
	Icon           string
	Version        string
	Script         string
	Config         string
	PollInterval   int
	PackageName    string
}

type paymentMethodPackageArchivePreview struct {
	Manifest     *marketPaymentPackageManifest
	ManifestRaw  string
	TargetMethod *models.PaymentMethod
	Resolved     *marketPaymentPackageResolved
}

func ImportPaymentMethodPackageArchive(
	runtime *PluginHostRuntime,
	packagePath string,
	packageName string,
	checksum string,
	params map[string]interface{},
) (map[string]interface{}, error) {
	return importPaymentMethodPackageArchiveWithOptions(runtime, packagePath, packageName, checksum, params, paymentMethodPackageImportOptions{})
}

func ImportLegacyPaymentMethodPackage(
	runtime *PluginHostRuntime,
	input legacyPaymentMethodImportInput,
) (map[string]interface{}, error) {
	definition, params, err := buildLegacyPaymentMethodPackageDefinition(input)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	archivePath, checksum, cleanup, err := createGeneratedPaymentPackageArchive(
		definition,
		"auralogic-legacy-payment-package-*.zip",
	)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "build legacy payment package failed"}
	}
	defer cleanup()

	return importPaymentMethodPackageArchiveWithOptions(
		runtime,
		archivePath,
		definition.PackageName,
		checksum,
		params,
		paymentMethodPackageImportOptions{},
	)
}

func PreviewPaymentMethodPackageArchive(
	runtime *PluginHostRuntime,
	packagePath string,
	checksum string,
	params map[string]interface{},
) (map[string]interface{}, error) {
	preview, err := previewPaymentMethodPackageArchive(
		runtime,
		packagePath,
		checksum,
		params,
		translateLocalPaymentMethodPackageManifestError,
	)
	if err != nil {
		return nil, err
	}

	db := runtime.database()
	return map[string]interface{}{
		"manifest":      preview.Manifest,
		"resolved":      preview.Resolved,
		"target_state":  buildMarketPaymentMethodTargetState(db, preview.TargetMethod, preview.Resolved.Version, preview.Resolved.Name),
		"webhook_count": len(preview.Manifest.Webhooks),
	}, nil
}

func importPaymentMethodPackageArchiveWithOptions(
	runtime *PluginHostRuntime,
	packagePath string,
	packageName string,
	checksum string,
	params map[string]interface{},
	options paymentMethodPackageImportOptions,
) (map[string]interface{}, error) {
	preview, err := previewPaymentMethodPackageArchive(runtime, packagePath, checksum, params, nil)
	if err != nil {
		return nil, err
	}
	db := runtime.database()
	if err := ensureMarketPaymentMethodNameAvailable(db, preview.Resolved.Name, marketPaymentMethodTargetID(preview.TargetMethod)); err != nil {
		return nil, err
	}

	normalizedPackageName := strings.TrimSpace(packageName)
	if normalizedPackageName == "" {
		normalizedPackageName = filepath.Base(strings.TrimSpace(packagePath))
	}
	normalizedPackageName = strings.TrimSpace(normalizedPackageName)
	if normalizedPackageName == "" {
		normalizedPackageName = "payment-package.zip"
	}

	created := preview.TargetMethod == nil
	var method *models.PaymentMethod
	var versionRecord *models.PaymentMethodVersion
	if err := db.Transaction(func(tx *gorm.DB) error {
		if created {
			method = &models.PaymentMethod{
				Name:            preview.Resolved.Name,
				Description:     preview.Resolved.Description,
				Type:            models.PaymentMethodTypeCustom,
				Enabled:         true,
				Script:          preview.Resolved.Script,
				Config:          preview.Resolved.Config,
				Icon:            preview.Resolved.Icon,
				Version:         preview.Resolved.Version,
				PackageName:     normalizedPackageName,
				PackageEntry:    preview.Resolved.Entry,
				PackageChecksum: checksum,
				Manifest:        preview.ManifestRaw,
				PollInterval:    preview.Resolved.PollInterval,
			}
			if err := createMarketPaymentMethod(tx, method); err != nil {
				return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create payment method failed"}
			}
		} else {
			updates := map[string]interface{}{
				"name":             preview.Resolved.Name,
				"description":      preview.Resolved.Description,
				"type":             models.PaymentMethodTypeCustom,
				"script":           preview.Resolved.Script,
				"config":           preview.Resolved.Config,
				"icon":             preview.Resolved.Icon,
				"version":          preview.Resolved.Version,
				"package_name":     normalizedPackageName,
				"package_entry":    preview.Resolved.Entry,
				"package_checksum": checksum,
				"manifest":         preview.ManifestRaw,
				"poll_interval":    preview.Resolved.PollInterval,
			}
			if err := tx.Model(&models.PaymentMethod{}).Where("id = ?", preview.TargetMethod.ID).Updates(updates).Error; err != nil {
				return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "update payment method failed"}
			}
			method = &models.PaymentMethod{}
			if err := tx.First(method, preview.TargetMethod.ID).Error; err != nil {
				return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "load payment method failed"}
			}
		}

		record, err := createPaymentMethodVersionSnapshot(tx, method, options.Coordinates, nil)
		if err != nil {
			return err
		}
		versionRecord = record
		return nil
	}); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"status":        "imported",
		"created":       created,
		"item":          method,
		"history_entry": buildPaymentMethodVersionSummary(versionRecord),
		"manifest":      preview.Manifest,
		"resolved":      preview.Resolved,
		"target_state":  buildMarketPaymentMethodTargetState(db, method, preview.Resolved.Version, preview.Resolved.Name),
	}
	if strings.TrimSpace(options.Coordinates.Kind) != "" ||
		strings.TrimSpace(options.Coordinates.Name) != "" ||
		strings.TrimSpace(options.Coordinates.SourceID) != "" ||
		strings.TrimSpace(options.Coordinates.Version) != "" {
		result["coordinates"] = options.Coordinates.Map()
	}
	if len(options.Source) > 0 {
		result["source"] = clonePluginMarketMap(options.Source)
	}
	if len(options.Release) > 0 {
		result["release"] = clonePluginMarketMap(options.Release)
	}
	if len(options.Governance) > 0 {
		result["governance"] = clonePluginMarketMap(options.Governance)
	}
	if len(options.Download) > 0 {
		result["download"] = clonePluginMarketMap(options.Download)
	}
	if len(options.Compatibility) > 0 {
		result["compatibility"] = clonePluginMarketMap(options.Compatibility)
	}
	if len(options.Warnings) > 0 {
		result["warnings"] = clonePluginMarketStringSlice(options.Warnings)
	}
	return result, nil
}

func previewPaymentMethodPackageArchive(
	runtime *PluginHostRuntime,
	packagePath string,
	checksum string,
	params map[string]interface{},
	manifestErrorTranslator func(error) error,
) (*paymentMethodPackageArchivePreview, error) {
	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}

	manifest, manifestRaw, err := readMarketPaymentPackageManifestFromPackage(packagePath)
	if err != nil {
		if manifestErrorTranslator != nil {
			return nil, manifestErrorTranslator(err)
		}
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	targetMethodID, err := parseMarketPaymentMethodID(params)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	targetMethod, err := loadMarketPaymentMethodTarget(db, targetMethodID)
	if err != nil {
		return nil, err
	}

	requestedEntry := strings.TrimSpace(parsePluginHostOptionalString(params, "entry"))
	resolved, err := resolveMarketPaymentPackage(packagePath, checksum, manifest, requestedEntry, targetMethod)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	resolved.Name = pluginHostMarketFirstNonEmpty(
		strings.TrimSpace(parsePluginHostOptionalString(params, "name", "payment_name", "paymentName")),
		resolved.Name,
		marketPaymentMethodTargetName(targetMethod),
	)
	if resolved.Name == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "payment method name is required"}
	}
	resolved.Description = pluginHostMarketFirstNonEmpty(
		strings.TrimSpace(parsePluginHostOptionalString(params, "description", "payment_description", "paymentDescription")),
		resolved.Description,
		marketPaymentMethodTargetDescription(targetMethod),
	)
	resolved.Version = pluginHostMarketNormalizeVersion(
		pluginHostMarketFirstNonEmpty(
			strings.TrimSpace(parsePluginHostOptionalString(params, "package_version", "packageVersion", "import_version", "importVersion")),
			strings.TrimSpace(parsePluginHostOptionalString(params, "version_override", "versionOverride")),
			resolved.Version,
			marketPaymentMethodTargetVersion(targetMethod),
		),
	)
	resolved.Icon = pluginHostMarketFirstNonEmpty(
		strings.TrimSpace(parsePluginHostOptionalString(params, "icon")),
		resolved.Icon,
		marketPaymentMethodTargetIcon(targetMethod),
		"CreditCard",
	)

	configJSON, err := normalizeMarketPaymentPackageConfigJSON(
		strings.TrimSpace(parsePluginHostOptionalString(params, "config")),
		manifest,
		marketPaymentMethodTargetConfig(targetMethod),
	)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid config json"}
	}
	configObject, err := parseMarketPaymentPackageJSONObjectString(configJSON)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid config json"}
	}
	if err := validateMarketPaymentPackageConfigAgainstSchema(configObject, manifest.ConfigSchema); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	resolved.Config = configJSON

	pollInterval, err := normalizeMarketPaymentPackagePollInterval(
		strings.TrimSpace(parsePluginHostOptionalString(params, "poll_interval", "pollInterval")),
		manifest,
		marketPaymentMethodTargetPollInterval(targetMethod),
	)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid poll_interval"}
	}
	resolved.PollInterval = pollInterval

	return &paymentMethodPackageArchivePreview{
		Manifest:     manifest,
		ManifestRaw:  manifestRaw,
		TargetMethod: targetMethod,
		Resolved:     resolved,
	}, nil
}

func initBuiltinPaymentMethodPackages(runtime *PluginHostRuntime) error {
	if err := migrateLegacyPaymentMethodPackages(runtime); err != nil {
		return err
	}

	definitions, err := builtinPaymentPackageDefinitions()
	if err != nil {
		return err
	}

	db := runtime.database()
	if db == nil {
		return fmt.Errorf("payment method database is unavailable")
	}

	for _, definition := range definitions {
		existing, err := findBuiltinPaymentMethodTarget(db, definition)
		if err != nil {
			return err
		}

		archivePath, checksum, cleanup, err := createBuiltinPaymentPackageArchive(definition)
		if err != nil {
			return fmt.Errorf("build builtin payment package %s failed: %w", definition.DisplayName, err)
		}

		err = func() error {
			defer cleanup()
			if shouldSkipBuiltinPaymentPackageImport(existing, definition, checksum) {
				logger.LogSystemOperation(db, "payment_method_init", "payment_method", optionalPaymentMethodID(existing), map[string]interface{}{
					"name":          definition.DisplayName,
					"package_name":  definition.PackageName,
					"version":       definition.Version,
					"package_type":  "payment_package",
					"package_scope": "builtin",
					"action":        "skipped",
				})
				return nil
			}

			params := map[string]interface{}{}
			if existing != nil && existing.ID > 0 {
				params["payment_method_id"] = existing.ID
				params["name"] = strings.TrimSpace(existing.Name)
				params["description"] = strings.TrimSpace(existing.Description)
				params["icon"] = strings.TrimSpace(existing.Icon)
				params["poll_interval"] = existing.PollInterval
			}

			result, importErr := importPaymentMethodPackageArchiveWithOptions(
				runtime,
				archivePath,
				definition.PackageName,
				checksum,
				params,
				paymentMethodPackageImportOptions{
					Coordinates: pluginHostMarketCoordinates{
						SourceID: "builtin",
						Kind:     "payment_package",
						Name:     definition.ArtifactName,
						Version:  definition.Version,
					},
				},
			)
			if importErr != nil {
				return fmt.Errorf("import builtin payment package %s failed: %w", definition.DisplayName, importErr)
			}

			method, _ := result["item"].(*models.PaymentMethod)
			methodID := optionalPaymentMethodID(method)
			action := "updated"
			if created, _ := result["created"].(bool); created {
				action = "created"
			}
			logger.LogSystemOperation(db, "payment_method_init", "payment_method", methodID, map[string]interface{}{
				"name":          definition.DisplayName,
				"package_name":  definition.PackageName,
				"version":       definition.Version,
				"package_type":  "payment_package",
				"package_scope": "builtin",
				"action":        action,
			})
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateLegacyPaymentMethodPackages(runtime *PluginHostRuntime) error {
	db := runtime.database()
	if db == nil {
		return fmt.Errorf("payment method database is unavailable")
	}

	var methods []models.PaymentMethod
	if err := db.Order("id ASC").Find(&methods).Error; err != nil {
		return err
	}

	for idx := range methods {
		method := methods[idx]
		if !shouldAutoMigrateLegacyPaymentMethod(&method) {
			continue
		}

		result, err := ImportLegacyPaymentMethodPackage(runtime, legacyPaymentMethodImportInput{
			TargetMethodID: method.ID,
			Name:           method.Name,
			Description:    method.Description,
			Icon:           method.Icon,
			Version:        method.Version,
			Script:         method.Script,
			Config:         method.Config,
			PollInterval:   method.PollInterval,
			PackageName:    method.PackageName,
		})
		if err != nil {
			logger.LogSystemOperation(db, "payment_method_legacy_migration", "payment_method", optionalPaymentMethodID(&method), map[string]interface{}{
				"name":          strings.TrimSpace(method.Name),
				"package_scope": "legacy_paymentjs",
				"action":        "failed",
				"error":         err.Error(),
			})
			continue
		}

		migrated, _ := result["item"].(*models.PaymentMethod)
		logger.LogSystemOperation(db, "payment_method_legacy_migration", "payment_method", optionalPaymentMethodID(migrated), map[string]interface{}{
			"name":          strings.TrimSpace(method.Name),
			"package_name":  strings.TrimSpace(optionalPaymentMethodPackageName(migrated, method.PackageName)),
			"version":       strings.TrimSpace(optionalPaymentMethodVersion(migrated, method.Version)),
			"package_scope": "legacy_paymentjs",
			"action":        "migrated",
		})
	}

	return nil
}

func builtinPaymentPackageDefinitions() ([]builtinPaymentPackageDefinition, error) {
	definitions := make([]builtinPaymentPackageDefinition, 0, len(models.BuiltinPaymentMethods))
	for _, method := range models.BuiltinPaymentMethods {
		artifactName := builtinPaymentPackageArtifactName(method.Name)
		if artifactName == "" {
			continue
		}

		configValues, err := parseMarketPaymentPackageJSONObjectString(strings.TrimSpace(method.Config))
		if err != nil {
			return nil, fmt.Errorf("builtin payment method %s config is invalid: %w", method.Name, err)
		}
		pollInterval := method.PollInterval
		if pollInterval <= 0 {
			pollInterval = 30
		}
		version := builtinPaymentPackageVersion(artifactName)
		definitions = append(definitions, builtinPaymentPackageDefinition{
			ArtifactName:  artifactName,
			DisplayName:   strings.TrimSpace(method.Name),
			Description:   strings.TrimSpace(method.Description),
			Icon:          strings.TrimSpace(method.Icon),
			Version:       version,
			PollInterval:  pollInterval,
			Config:        configValues,
			Script:        method.Script,
			PackageName:   fmt.Sprintf("%s-%s.zip", artifactName, version),
			EntryFileName: "index.js",
		})
	}
	return definitions, nil
}

func builtinPaymentPackageArtifactName(name string) string {
	switch strings.TrimSpace(name) {
	case "USDT TRC20":
		return "builtin-usdt-trc20"
	case "USDT BEP20 (BSC)":
		return "builtin-usdt-bep20-bsc"
	default:
		return ""
	}
}

func builtinPaymentPackageVersion(_ string) string {
	return "1.0.0"
}

func findBuiltinPaymentMethodTarget(db *gorm.DB, definition builtinPaymentPackageDefinition) (*models.PaymentMethod, error) {
	if db == nil {
		return nil, fmt.Errorf("payment method database is unavailable")
	}

	var existing models.PaymentMethod
	if err := db.Where("package_name = ?", definition.PackageName).First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := db.Where("name = ?", definition.DisplayName).First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, nil
}

func shouldSkipBuiltinPaymentPackageImport(
	existing *models.PaymentMethod,
	definition builtinPaymentPackageDefinition,
	checksum string,
) bool {
	if existing == nil || existing.ID == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(existing.PackageName), strings.TrimSpace(definition.PackageName)) &&
		strings.EqualFold(strings.TrimSpace(existing.PackageChecksum), strings.TrimSpace(checksum)) &&
		pluginHostMarketNormalizeVersion(strings.TrimSpace(existing.Version)) == pluginHostMarketNormalizeVersion(strings.TrimSpace(definition.Version)) &&
		strings.TrimSpace(existing.PackageEntry) != "" &&
		strings.TrimSpace(existing.Manifest) != ""
}

func shouldAutoMigrateLegacyPaymentMethod(method *models.PaymentMethod) bool {
	if method == nil || method.ID == 0 {
		return false
	}
	if method.Type != models.PaymentMethodTypeCustom {
		return false
	}
	if builtinPaymentPackageArtifactName(method.Name) != "" {
		return false
	}
	return strings.TrimSpace(method.PackageName) == "" ||
		strings.TrimSpace(method.PackageEntry) == "" ||
		strings.TrimSpace(method.PackageChecksum) == "" ||
		strings.TrimSpace(method.Manifest) == ""
}

func createBuiltinPaymentPackageArchive(
	definition builtinPaymentPackageDefinition,
) (string, string, func(), error) {
	return createGeneratedPaymentPackageArchive(definition, "auralogic-builtin-payment-package-*.zip")
}

func createGeneratedPaymentPackageArchive(
	definition generatedPaymentMethodPackageDefinition,
	tempPattern string,
) (string, string, func(), error) {
	pollInterval := definition.PollInterval
	if pollInterval <= 0 {
		pollInterval = 30
	}
	manifest := marketPaymentPackageManifest{
		Name:                   definition.ArtifactName,
		DisplayName:            ManifestLocalizedText{raw: definition.DisplayName, value: strings.TrimSpace(definition.DisplayName)},
		Description:            ManifestLocalizedText{raw: definition.Description, value: strings.TrimSpace(definition.Description)},
		Icon:                   definition.Icon,
		Runtime:                "payment_js",
		Entry:                  definition.EntryFileName,
		Version:                definition.Version,
		PollInterval:           &pollInterval,
		ManifestVersion:        PluginHostManifestVersion,
		ProtocolVersion:        DefaultPluginHostProtocolVersion,
		MinHostProtocolVersion: DefaultPluginHostProtocolVersion,
		MaxHostProtocolVersion: DefaultPluginHostProtocolVersion,
		Config:                 definition.Config,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return "", "", nil, err
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	if err := writeGeneratedPaymentPackageZipFile(writer, "manifest.json", manifestRaw); err != nil {
		_ = writer.Close()
		return "", "", nil, err
	}
	if err := writeGeneratedPaymentPackageZipFile(writer, definition.EntryFileName, []byte(definition.Script)); err != nil {
		_ = writer.Close()
		return "", "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", "", nil, err
	}

	tempFile, err := os.CreateTemp("", tempPattern)
	if err != nil {
		return "", "", nil, err
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(archive.Bytes()); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", "", nil, err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", "", nil, err
	}

	checksum := sha256.Sum256(archive.Bytes())
	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	return filepath.ToSlash(tempPath), hex.EncodeToString(checksum[:]), cleanup, nil
}

func writeGeneratedPaymentPackageZipFile(writer *zip.Writer, name string, body []byte) error {
	header := &zip.FileHeader{
		Name:   strings.TrimSpace(name),
		Method: zip.Deflate,
	}
	header.SetModTime(time.Unix(0, 0).UTC())
	entryWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entryWriter.Write(body)
	return err
}

func optionalPaymentMethodID(method *models.PaymentMethod) *uint {
	if method == nil || method.ID == 0 {
		return nil
	}
	id := method.ID
	return &id
}

func buildLegacyPaymentMethodPackageDefinition(
	input legacyPaymentMethodImportInput,
) (generatedPaymentMethodPackageDefinition, map[string]interface{}, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return generatedPaymentMethodPackageDefinition{}, nil, fmt.Errorf("payment method name is required")
	}

	configRaw := strings.TrimSpace(input.Config)
	configValues, err := parseMarketPaymentPackageJSONObjectString(configRaw)
	if err != nil {
		return generatedPaymentMethodPackageDefinition{}, nil, fmt.Errorf("invalid config json")
	}
	if configValues == nil {
		configValues = map[string]interface{}{}
	}

	pollInterval := input.PollInterval
	if pollInterval <= 0 {
		pollInterval = 30
	}
	version := pluginHostMarketNormalizeVersion(
		pluginHostMarketFirstNonEmpty(strings.TrimSpace(input.Version), "1.0.0"),
	)
	icon := pluginHostMarketFirstNonEmpty(strings.TrimSpace(input.Icon), "CreditCard")
	artifactName := legacyPaymentMethodArtifactName(name, input.PackageName, input.TargetMethodID)
	packageName := legacyPaymentMethodPackageName(input.PackageName, artifactName)

	params := map[string]interface{}{
		"name":            name,
		"description":     strings.TrimSpace(input.Description),
		"icon":            icon,
		"package_version": version,
		"poll_interval":   pollInterval,
	}
	if input.TargetMethodID > 0 {
		params["payment_method_id"] = input.TargetMethodID
	}
	if configRaw != "" {
		params["config"] = configRaw
	}

	return generatedPaymentMethodPackageDefinition{
		ArtifactName:  artifactName,
		DisplayName:   name,
		Description:   strings.TrimSpace(input.Description),
		Icon:          icon,
		Version:       version,
		PollInterval:  pollInterval,
		Config:        configValues,
		Script:        input.Script,
		PackageName:   packageName,
		EntryFileName: "index.js",
	}, params, nil
}

func legacyPaymentMethodArtifactName(name string, packageName string, targetMethodID uint) string {
	if trimmedPackageName := strings.TrimSpace(packageName); trimmedPackageName != "" {
		baseName := strings.TrimSuffix(filepath.Base(trimmedPackageName), filepath.Ext(trimmedPackageName))
		sanitized := sanitizeJSWorkerPathSegment(strings.ToLower(strings.TrimSpace(baseName)))
		if sanitized != "" {
			return sanitized
		}
	}

	sanitizedName := sanitizeJSWorkerPathSegment(strings.ToLower(strings.TrimSpace(name)))
	if sanitizedName == "" {
		sanitizedName = "payment-method"
	}
	if strings.HasPrefix(sanitizedName, "legacy-paymentjs-") {
		return sanitizedName
	}
	if targetMethodID > 0 {
		return fmt.Sprintf("legacy-paymentjs-%d-%s", targetMethodID, sanitizedName)
	}
	return "legacy-paymentjs-" + sanitizedName
}

func legacyPaymentMethodPackageName(rawPackageName string, artifactName string) string {
	trimmed := strings.TrimSpace(rawPackageName)
	if trimmed == "" {
		trimmed = artifactName + ".zip"
	}
	baseName := filepath.Base(trimmed)
	if strings.EqualFold(filepath.Ext(baseName), ".zip") {
		return baseName
	}
	return baseName + ".zip"
}

func optionalPaymentMethodPackageName(method *models.PaymentMethod, fallback string) string {
	if method == nil {
		return strings.TrimSpace(fallback)
	}
	return pluginHostMarketFirstNonEmpty(strings.TrimSpace(method.PackageName), strings.TrimSpace(fallback))
}

func optionalPaymentMethodVersion(method *models.PaymentMethod, fallback string) string {
	if method == nil {
		return strings.TrimSpace(fallback)
	}
	return pluginHostMarketFirstNonEmpty(strings.TrimSpace(method.Version), strings.TrimSpace(fallback))
}

func translateLocalPaymentMethodPackageManifestError(err error) error {
	if err == nil {
		return nil
	}

	var typedBizErr *bizerr.Error
	if errors.As(err, &typedBizErr) {
		return err
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		return err
	}

	const invalidJSONPrefix = "market payment manifest is invalid json:"
	if strings.HasPrefix(message, invalidJSONPrefix) {
		cause := strings.TrimSpace(strings.TrimPrefix(message, invalidJSONPrefix))
		params := map[string]interface{}{}
		if cause != "" {
			params["cause"] = cause
		}
		return bizerr.New(
			"plugin.admin.http_400.invalid_package_manifest_json",
			"invalid package manifest json",
		).WithParams(params)
	}

	const compatibilityPrefix = "payment package manifest compatibility is invalid:"
	if strings.HasPrefix(message, compatibilityPrefix) {
		reason := strings.TrimSpace(strings.TrimPrefix(message, compatibilityPrefix))
		return bizerr.New(
			"plugin.admin.http_400.invalid_package_manifest_schema",
			"invalid package manifest schema",
		).WithParams(map[string]interface{}{
			"path":   "manifest",
			"reason": reason,
		})
	}

	path, reason, ok := splitLocalPaymentMethodManifestSchemaError(message)
	if ok {
		return bizerr.New(
			"plugin.admin.http_400.invalid_package_manifest_schema",
			"invalid package manifest schema",
		).WithParams(map[string]interface{}{
			"path":   path,
			"reason": reason,
		})
	}

	return err
}

func splitLocalPaymentMethodManifestSchemaError(message string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(message), " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	path := strings.TrimSpace(parts[0])
	reason := strings.TrimSpace(parts[1])
	if path == "" || reason == "" {
		return "", "", false
	}
	if !strings.Contains(path, ".") && !strings.Contains(path, "[") {
		return "", "", false
	}
	return path, reason, true
}
