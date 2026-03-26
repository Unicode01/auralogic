package registrycli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"auralogic/market_registry/pkg/registryruntime"
	"auralogic/market_registry/pkg/runtimeconfig"
)

type App struct {
	Stdout io.Writer
	Stderr io.Writer
}

const shellModulePath = runtimeconfig.ModulePath

func New() App {
	return App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (app App) Run(args []string) int {
	if len(args) < 2 {
		app.printUsage()
		return 1
	}

	switch args[1] {
	case "audit":
		return app.handleAudit(args)
	case "keygen":
		return app.handleKeygen(args)
	case "pull":
		return app.handlePull(args)
	case "publish":
		return app.handlePublish(args)
	case "sync":
		return app.handleSync(args)
	case "config", "env", "aliases":
		return app.handleConfig(args)
	case "reindex", "repair":
		return app.handleReindex()
	default:
		fmt.Fprintf(app.Stderr, "Unknown command: %s\n", args[1])
		app.printUsage()
		return 1
	}
}

func (app App) printUsage() {
	fmt.Fprintln(app.Stdout, "Usage:")
	fmt.Fprintln(app.Stdout, "  market-registry-cli audit [--json]")
	fmt.Fprintln(app.Stdout, "  market-registry-cli keygen --key-id <id>")
	fmt.Fprintln(app.Stdout, "  market-registry-cli pull --kind <kind> --name <name> [--version <version>] [--channel <channel>] [--output <file-or-dir>] [--source-url <url>] [--force]")
	fmt.Fprintln(app.Stdout, "  market-registry-cli publish --artifact <file> [--kind <kind>] [--name <name>] [--version <version>] [--metadata <file>] [--manifest <file>]")
	fmt.Fprintln(app.Stdout, "  market-registry-cli sync github-release --owner <owner> --repo <repo> --tag <tag> --asset <asset> [--kind <kind>] [--name <name>] [--version <version>] [--metadata <file>] [--manifest <file>]")
	fmt.Fprintln(app.Stdout, "  market-registry-cli reindex")
	fmt.Fprintln(app.Stdout, "  market-registry-cli config [--json] [--api|--shared]")
}

func (app App) handleKeygen(args []string) int {
	keyID := getFlag(args, "--key-id")
	if keyID == "" {
		fmt.Fprintln(app.Stderr, "Error: --key-id required")
		return 1
	}

	cfg := runtimeconfig.LoadShared()
	cliRuntime, err := registryruntime.NewCLI(runtimeconfig.CLI{Shared: cfg})
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error initializing runtime: %v\n", err)
		return 1
	}

	keyPair, err := cliRuntime.GenerateKeyPair(keyID)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(app.Stdout, "Generated key pair: %s\n", keyPair.KeyID)
	fmt.Fprintf(app.Stdout, "Public key: %s\n", keyPair.PublicKey)
	fmt.Fprintf(app.Stdout, "Private key saved to: %s\n", keyPair.PrivateKeyPath)
	return 0
}

func (app App) handlePublish(args []string) int {
	artifactFile := getFlag(args, "--artifact")
	if artifactFile == "" {
		fmt.Fprintln(app.Stderr, "Error: --artifact required")
		return 1
	}

	zipData, err := os.ReadFile(artifactFile)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error reading artifact: %v\n", err)
		return 1
	}

	metadata, err := loadMetadataFile(getFlag(args, "--metadata"))
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error loading metadata: %v\n", err)
		return 1
	}
	localManifest, err := loadOptionalLocalManifest(getFlag(args, "--manifest"))
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error loading local manifest: %v\n", err)
		return 1
	}
	kind, name, version, metadata := applyLocalManifestAutofill(
		getFlag(args, "--kind"),
		getFlag(args, "--name"),
		getFlag(args, "--version"),
		metadata,
		localManifest,
	)

	cfg := runtimeconfig.LoadCLI()
	cliRuntime, err := registryruntime.NewCLI(cfg)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error initializing runtime: %v\n", err)
		return 1
	}
	req := registryruntime.PublishRequest{
		Kind:        kind,
		Name:        name,
		Version:     version,
		Channel:     cfg.Channel,
		ArtifactZip: zipData,
		Metadata:    metadata,
	}
	if err := cliRuntime.Publish(context.Background(), req); err != nil {
		fmt.Fprintf(app.Stderr, "Error publishing: %v\n", err)
		return 1
	}

	fmt.Fprintf(app.Stdout, "Successfully published %s:%s:%s\n", kind, name, version)
	if result, err := cliRuntime.RebuildRegistry(context.Background()); err != nil {
		fmt.Fprintf(app.Stdout, "Warning: published, but reindex failed: %v\n", err)
	} else {
		fmt.Fprintf(app.Stdout, "Registry snapshots rebuilt: %s, %s (%d artifacts)\n", result.SourcePath, result.CatalogPath, result.TotalArtifacts)
	}
	return 0
}

func (app App) handleSync(args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(app.Stderr, "Error: sync subcommand required")
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(args[2])) {
	case "github-release":
		return app.handleSyncGitHubRelease(args)
	default:
		fmt.Fprintf(app.Stderr, "Error: unknown sync subcommand %q\n", strings.TrimSpace(args[2]))
		return 1
	}
}

func (app App) handleSyncGitHubRelease(args []string) int {
	owner := getFlag(args, "--owner")
	repo := getFlag(args, "--repo")
	tag := getFlag(args, "--tag")
	assetName := getFlag(args, "--asset")
	if owner == "" || repo == "" || tag == "" || assetName == "" {
		fmt.Fprintln(app.Stderr, "Error: --owner, --repo, --tag, --asset required")
		return 1
	}

	metadata, err := loadMetadataFile(getFlag(args, "--metadata"))
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error loading metadata: %v\n", err)
		return 1
	}
	localManifest, err := loadOptionalLocalManifest(getFlag(args, "--manifest"))
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error loading local manifest: %v\n", err)
		return 1
	}
	kind, name, version, metadata := applyLocalManifestAutofill(
		getFlag(args, "--kind"),
		getFlag(args, "--name"),
		getFlag(args, "--version"),
		metadata,
		localManifest,
	)

	cfg := runtimeconfig.LoadCLI()
	cliRuntime, err := registryruntime.NewCLI(cfg)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error initializing runtime: %v\n", err)
		return 1
	}

	result, err := cliRuntime.SyncGitHubRelease(context.Background(), registryruntime.GitHubReleaseSyncRequest{
		Kind:       kind,
		Name:       name,
		Version:    version,
		Channel:    getFlag(args, "--channel"),
		Owner:      owner,
		Repo:       repo,
		Tag:        tag,
		AssetName:  assetName,
		APIBaseURL: getFlag(args, "--api-base-url"),
		Token:      resolveGitHubToken(getFlag(args, "--token-env")),
		Metadata:   metadata,
	})
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error syncing github release: %v\n", err)
		return 1
	}

	fmt.Fprintf(app.Stdout, "Successfully synced GitHub release %s/%s@%s asset %s\n", result.Owner, result.Repo, result.Tag, result.AssetName)
	fmt.Fprintf(app.Stdout, "Published as %s:%s:%s\n", result.Kind, result.Name, result.Version)
	fmt.Fprintf(app.Stdout, "SHA256: %s\n", result.SHA256)
	fmt.Fprintf(app.Stdout, "Asset size: %d\n", result.AssetSize)

	if rebuild, rebuildErr := cliRuntime.RebuildRegistry(context.Background()); rebuildErr != nil {
		fmt.Fprintf(app.Stdout, "Warning: synced, but reindex failed: %v\n", rebuildErr)
	} else {
		fmt.Fprintf(app.Stdout, "Registry snapshots rebuilt: %s, %s (%d artifacts)\n", rebuild.SourcePath, rebuild.CatalogPath, rebuild.TotalArtifacts)
	}
	return 0
}

func (app App) handleReindex() int {
	cfg := runtimeconfig.LoadCLI()
	cliRuntime, err := registryruntime.NewCLI(cfg)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error initializing runtime: %v\n", err)
		return 1
	}

	result, err := cliRuntime.RebuildRegistry(context.Background())
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error rebuilding registry: %v\n", err)
		return 1
	}

	fmt.Fprintln(app.Stdout, "Registry rebuilt successfully.")
	fmt.Fprintf(app.Stdout, "Source snapshot: %s\n", result.SourcePath)
	fmt.Fprintf(app.Stdout, "Catalog snapshot: %s\n", result.CatalogPath)
	fmt.Fprintf(app.Stdout, "Artifacts indexed: %d\n", result.TotalArtifacts)
	if result.GeneratedAt != "" {
		fmt.Fprintf(app.Stdout, "Generated at: %s\n", result.GeneratedAt)
	}
	return 0
}

func (app App) handleConfig(args []string) int {
	scope := "cli"
	if hasFlag(args, "--api") {
		scope = "api"
	} else if hasFlag(args, "--shared") {
		scope = "shared"
	}

	payload := app.buildConfigPayload(scope)
	if hasFlag(args, "--json") {
		encoder := json.NewEncoder(app.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintf(app.Stderr, "Error encoding config: %v\n", err)
			return 1
		}
		return 0
	}

	app.printConfigPayload(payload)
	return 0
}

func (app App) buildConfigPayload(scope string) map[string]any {
	payload := map[string]any{
		"module": runtimeconfig.ModulePath,
		"modules": map[string]any{
			"implementation": runtimeconfig.ModulePath,
			"shell":          shellModulePath,
		},
		"scope": scope,
	}

	switch scope {
	case "api":
		cfg := runtimeconfig.LoadAPI()
		payload["runtime"] = map[string]any{
			"data_dir":                         cfg.Shared.DataDir,
			"key_dir":                          cfg.Shared.KeyDir,
			"base_url":                         cfg.Shared.BaseURL,
			"key_id":                           cfg.Shared.KeyID,
			"source_id":                        cfg.Shared.SourceID,
			"source_name":                      cfg.Shared.SourceName,
			"storage_type":                     cfg.Shared.StorageType,
			"storage_s3_endpoint":              cfg.Shared.StorageS3Endpoint,
			"storage_s3_region":                cfg.Shared.StorageS3Region,
			"storage_s3_bucket":                cfg.Shared.StorageS3Bucket,
			"storage_s3_prefix":                cfg.Shared.StorageS3Prefix,
			"storage_s3_use_path_style":        cfg.Shared.StorageS3UsePathStyle,
			"storage_s3_access_key_id_set":     cfg.Shared.StorageS3AccessKeyID != "",
			"storage_s3_secret_access_key_set": cfg.Shared.StorageS3SecretAccessKey != "",
			"storage_s3_session_token_set":     cfg.Shared.StorageS3SessionToken != "",
			"storage_webdav_endpoint":          cfg.Shared.StorageWebDAVEndpoint,
			"storage_webdav_username":          cfg.Shared.StorageWebDAVUsername,
			"storage_webdav_password_set":      cfg.Shared.StorageWebDAVPassword != "",
			"storage_webdav_skip_verify":       cfg.Shared.StorageWebDAVSkipVerify,
			"storage_ftp_address":              cfg.Shared.StorageFTPAddress,
			"storage_ftp_username":             cfg.Shared.StorageFTPUsername,
			"storage_ftp_password_set":         cfg.Shared.StorageFTPPassword != "",
			"storage_ftp_root_dir":             cfg.Shared.StorageFTPRootDir,
			"storage_ftp_security":             cfg.Shared.StorageFTPSecurity,
			"storage_ftp_skip_verify":          cfg.Shared.StorageFTPSkipVerify,
			"addr":                             cfg.Addr,
			"admin_username":                   cfg.AdminUsername,
			"admin_password_set":               cfg.AdminPassword != "",
			"admin_password_hash_set":          cfg.AdminPasswordHash != "",
			"token_ttl":                        cfg.TokenTTL.String(),
		}
		payload["bindings"] = runtimeconfig.APIBindings()
		payload["commands"] = runtimeconfig.APICommandBinding()
		payload["filesystem"] = runtimeconfig.APIFilesystemBinding()
	case "shared":
		cfg := runtimeconfig.LoadShared()
		payload["runtime"] = map[string]any{
			"data_dir":                         cfg.DataDir,
			"key_dir":                          cfg.KeyDir,
			"base_url":                         cfg.BaseURL,
			"key_id":                           cfg.KeyID,
			"source_id":                        cfg.SourceID,
			"source_name":                      cfg.SourceName,
			"storage_type":                     cfg.StorageType,
			"storage_s3_endpoint":              cfg.StorageS3Endpoint,
			"storage_s3_region":                cfg.StorageS3Region,
			"storage_s3_bucket":                cfg.StorageS3Bucket,
			"storage_s3_prefix":                cfg.StorageS3Prefix,
			"storage_s3_use_path_style":        cfg.StorageS3UsePathStyle,
			"storage_s3_access_key_id_set":     cfg.StorageS3AccessKeyID != "",
			"storage_s3_secret_access_key_set": cfg.StorageS3SecretAccessKey != "",
			"storage_s3_session_token_set":     cfg.StorageS3SessionToken != "",
			"storage_webdav_endpoint":          cfg.StorageWebDAVEndpoint,
			"storage_webdav_username":          cfg.StorageWebDAVUsername,
			"storage_webdav_password_set":      cfg.StorageWebDAVPassword != "",
			"storage_webdav_skip_verify":       cfg.StorageWebDAVSkipVerify,
			"storage_ftp_address":              cfg.StorageFTPAddress,
			"storage_ftp_username":             cfg.StorageFTPUsername,
			"storage_ftp_password_set":         cfg.StorageFTPPassword != "",
			"storage_ftp_root_dir":             cfg.StorageFTPRootDir,
			"storage_ftp_security":             cfg.StorageFTPSecurity,
			"storage_ftp_skip_verify":          cfg.StorageFTPSkipVerify,
		}
		payload["bindings"] = runtimeconfig.SharedBindings()
		payload["commands"] = map[string]any{
			"api": runtimeconfig.APICommandBinding(),
			"cli": runtimeconfig.CLICommandBinding(),
		}
		payload["filesystem"] = map[string]any{
			"api": runtimeconfig.APIFilesystemBinding(),
			"cli": runtimeconfig.CLIFilesystemBinding(),
		}
	default:
		cfg := runtimeconfig.LoadCLI()
		payload["runtime"] = map[string]any{
			"data_dir":                         cfg.Shared.DataDir,
			"key_dir":                          cfg.Shared.KeyDir,
			"base_url":                         cfg.Shared.BaseURL,
			"key_id":                           cfg.Shared.KeyID,
			"source_id":                        cfg.Shared.SourceID,
			"source_name":                      cfg.Shared.SourceName,
			"storage_type":                     cfg.Shared.StorageType,
			"storage_s3_endpoint":              cfg.Shared.StorageS3Endpoint,
			"storage_s3_region":                cfg.Shared.StorageS3Region,
			"storage_s3_bucket":                cfg.Shared.StorageS3Bucket,
			"storage_s3_prefix":                cfg.Shared.StorageS3Prefix,
			"storage_s3_use_path_style":        cfg.Shared.StorageS3UsePathStyle,
			"storage_s3_access_key_id_set":     cfg.Shared.StorageS3AccessKeyID != "",
			"storage_s3_secret_access_key_set": cfg.Shared.StorageS3SecretAccessKey != "",
			"storage_s3_session_token_set":     cfg.Shared.StorageS3SessionToken != "",
			"storage_webdav_endpoint":          cfg.Shared.StorageWebDAVEndpoint,
			"storage_webdav_username":          cfg.Shared.StorageWebDAVUsername,
			"storage_webdav_password_set":      cfg.Shared.StorageWebDAVPassword != "",
			"storage_webdav_skip_verify":       cfg.Shared.StorageWebDAVSkipVerify,
			"storage_ftp_address":              cfg.Shared.StorageFTPAddress,
			"storage_ftp_username":             cfg.Shared.StorageFTPUsername,
			"storage_ftp_password_set":         cfg.Shared.StorageFTPPassword != "",
			"storage_ftp_root_dir":             cfg.Shared.StorageFTPRootDir,
			"storage_ftp_security":             cfg.Shared.StorageFTPSecurity,
			"storage_ftp_skip_verify":          cfg.Shared.StorageFTPSkipVerify,
			"channel":                          cfg.Channel,
		}
		payload["bindings"] = runtimeconfig.CLIBindings()
		payload["commands"] = runtimeconfig.CLICommandBinding()
		payload["filesystem"] = runtimeconfig.CLIFilesystemBinding()
	}

	return payload
}

func (app App) printConfigPayload(payload map[string]any) {
	scope, _ := payload["scope"].(string)
	fmt.Fprintf(app.Stdout, "Implementation module: %s\n", runtimeconfig.ModulePath)
	fmt.Fprintf(app.Stdout, "Shell module: %s\n", shellModulePath)
	fmt.Fprintf(app.Stdout, "Scope: %s\n", scope)
	fmt.Fprintln(app.Stdout, "")

	if commands, ok := payload["commands"].(runtimeconfig.CommandBinding); ok {
		app.printCommandBinding(commands)
	} else if commands, ok := payload["commands"].(map[string]any); ok {
		fmt.Fprintln(app.Stdout, "Command bindings:")
		for _, key := range []string{"api", "cli"} {
			item, exists := commands[key]
			if !exists {
				continue
			}
			binding, _ := item.(runtimeconfig.CommandBinding)
			fmt.Fprintf(app.Stdout, "  %s:\n", key)
			app.printCommandBindingIndented("    ", binding)
		}
	}

	if filesystem, ok := payload["filesystem"].(runtimeconfig.FilesystemBinding); ok {
		app.printFilesystemBinding(filesystem)
	} else if filesystem, ok := payload["filesystem"].(map[string]any); ok {
		fmt.Fprintln(app.Stdout, "Filesystem bindings:")
		for _, key := range []string{"api", "cli"} {
			item, exists := filesystem[key]
			if !exists {
				continue
			}
			binding, _ := item.(runtimeconfig.FilesystemBinding)
			fmt.Fprintf(app.Stdout, "  %s:\n", key)
			app.printFilesystemBindingIndented("    ", binding)
		}
	}

	if runtimeData, ok := payload["runtime"].(map[string]any); ok {
		fmt.Fprintln(app.Stdout, "Resolved runtime:")
		for _, key := range []string{
			"data_dir",
			"key_dir",
			"base_url",
			"key_id",
			"source_id",
			"source_name",
			"storage_type",
			"storage_s3_endpoint",
			"storage_s3_region",
			"storage_s3_bucket",
			"storage_s3_prefix",
			"storage_s3_use_path_style",
			"storage_s3_access_key_id_set",
			"storage_s3_secret_access_key_set",
			"storage_s3_session_token_set",
			"storage_webdav_endpoint",
			"storage_webdav_username",
			"storage_webdav_password_set",
			"storage_webdav_skip_verify",
			"storage_ftp_address",
			"storage_ftp_username",
			"storage_ftp_password_set",
			"storage_ftp_root_dir",
			"storage_ftp_security",
			"storage_ftp_skip_verify",
			"channel",
			"addr",
			"admin_username",
			"admin_password_set",
			"admin_password_hash_set",
			"token_ttl",
		} {
			value, exists := runtimeData[key]
			if !exists {
				continue
			}
			fmt.Fprintf(app.Stdout, "  %s: %v\n", key, value)
		}
	}

	if bindings, ok := payload["bindings"].([]runtimeconfig.EnvBinding); ok {
		fmt.Fprintln(app.Stdout, "")
		fmt.Fprintln(app.Stdout, "Environment aliases:")
		for _, binding := range bindings {
			if len(binding.Legacy) == 0 {
				fmt.Fprintf(app.Stdout, "  %s", binding.Canonical)
			} else {
				fmt.Fprintf(app.Stdout, "  %s (legacy: %s)", binding.Canonical, strings.Join(binding.Legacy, ", "))
			}
			if binding.Description != "" {
				fmt.Fprintf(app.Stdout, "  # %s", binding.Description)
			}
			fmt.Fprintln(app.Stdout)
		}
	}
}

func (app App) printCommandBinding(binding runtimeconfig.CommandBinding) {
	fmt.Fprintln(app.Stdout, "Command binding:")
	app.printCommandBindingIndented("  ", binding)
}

func (app App) printCommandBindingIndented(prefix string, binding runtimeconfig.CommandBinding) {
	if len(binding.Canonical) > 0 {
		fmt.Fprintf(app.Stdout, "%scanonical: %s\n", prefix, strings.Join(binding.Canonical, ", "))
	}
	if len(binding.Legacy) > 0 {
		fmt.Fprintf(app.Stdout, "%slegacy: %s\n", prefix, strings.Join(binding.Legacy, ", "))
	}
}

func (app App) printFilesystemBinding(binding runtimeconfig.FilesystemBinding) {
	fmt.Fprintln(app.Stdout, "Filesystem binding:")
	app.printFilesystemBindingIndented("  ", binding)
}

func (app App) printFilesystemBindingIndented(prefix string, binding runtimeconfig.FilesystemBinding) {
	if len(binding.Canonical) > 0 {
		fmt.Fprintf(app.Stdout, "%scanonical: %s\n", prefix, strings.Join(binding.Canonical, ", "))
	}
	if len(binding.Legacy) > 0 {
		fmt.Fprintf(app.Stdout, "%slegacy: %s\n", prefix, strings.Join(binding.Legacy, ", "))
	}
}

func getFlag(args []string, name string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func loadMetadataFile(path string) (registryruntime.Metadata, error) {
	if strings.TrimSpace(path) == "" {
		return registryruntime.Metadata{}, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return registryruntime.Metadata{}, err
	}
	var metadata registryruntime.Metadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return registryruntime.Metadata{}, err
	}
	return metadata, nil
}

func resolveGitHubToken(tokenEnv string) string {
	envNames := []string{}
	if trimmed := strings.TrimSpace(tokenEnv); trimmed != "" {
		envNames = append(envNames, trimmed)
	}
	envNames = append(envNames, "MARKET_REGISTRY_GITHUB_TOKEN", "GITHUB_TOKEN")
	for _, name := range envNames {
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}

func (app App) writeJSON(value any) error {
	encoder := json.NewEncoder(app.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
