package admin

import (
	"auralogic/internal/models"
	"auralogic/internal/service"
	"path/filepath"
	"strings"
)

type pluginDeploymentResponse struct {
	models.PluginDeployment
}

func buildPluginDeploymentResponse(deployment models.PluginDeployment) pluginDeploymentResponse {
	return pluginDeploymentResponse{PluginDeployment: deployment}
}

func buildPluginDeploymentResponses(deployments []models.PluginDeployment) []pluginDeploymentResponse {
	if len(deployments) == 0 {
		return []pluginDeploymentResponse{}
	}
	out := make([]pluginDeploymentResponse, 0, len(deployments))
	for _, deployment := range deployments {
		out = append(out, buildPluginDeploymentResponse(deployment))
	}
	return out
}

type pluginResponse struct {
	models.Plugin
	PackagePathDisplay        string                                  `json:"package_path_display,omitempty"`
	AddressDisplay            string                                  `json:"address_display,omitempty"`
	EffectiveCapabilityPolicy service.EffectivePluginCapabilityPolicy `json:"effective_capability_policy"`
	WorkspaceCommands         []service.PluginWorkspaceCommand        `json:"workspace_commands,omitempty"`
	LatestDeployment          *pluginDeploymentResponse               `json:"latest_deployment,omitempty"`
}

func buildPluginResponse(plugin models.Plugin, artifactRoot string) pluginResponse {
	return buildPluginResponseWithDeployment(plugin, nil, artifactRoot)
}

func buildPluginResponseWithDeployment(
	plugin models.Plugin,
	latest *models.PluginDeployment,
	artifactRoot string,
) pluginResponse {
	var latestResp *pluginDeploymentResponse
	if latest != nil {
		copied := *latest
		latestResp = &pluginDeploymentResponse{PluginDeployment: copied}
	}
	return pluginResponse{
		Plugin:                    plugin,
		PackagePathDisplay:        buildPluginPackagePathDisplay(plugin.PackagePath, artifactRoot),
		AddressDisplay:            buildPluginAddressDisplay(plugin.Runtime, plugin.Address, artifactRoot),
		EffectiveCapabilityPolicy: service.ResolveEffectivePluginCapabilityPolicy(&plugin),
		WorkspaceCommands:         service.ResolvePluginWorkspaceCommands(&plugin),
		LatestDeployment:          latestResp,
	}
}

func buildPluginResponses(plugins []models.Plugin, artifactRoot string) []pluginResponse {
	return buildPluginResponsesWithLatestDeployments(plugins, nil, artifactRoot)
}

func buildPluginResponsesWithLatestDeployments(
	plugins []models.Plugin,
	latest map[uint]models.PluginDeployment,
	artifactRoot string,
) []pluginResponse {
	if len(plugins) == 0 {
		return []pluginResponse{}
	}
	out := make([]pluginResponse, 0, len(plugins))
	for idx := range plugins {
		var deployment *models.PluginDeployment
		if latest != nil {
			if record, exists := latest[plugins[idx].ID]; exists {
				copied := record
				deployment = &copied
			}
		}
		out = append(out, buildPluginResponseWithDeployment(plugins[idx], deployment, artifactRoot))
	}
	return out
}

type pluginVersionResponse struct {
	models.PluginVersion
	PackagePathDisplay string `json:"package_path_display,omitempty"`
}

func normalizeActivePluginVersionLifecycle(
	version models.PluginVersion,
	activeLifecycle string,
) models.PluginVersion {
	if !version.IsActive {
		return version
	}
	normalizedLifecycle := strings.TrimSpace(activeLifecycle)
	if normalizedLifecycle == "" {
		return version
	}
	version.LifecycleStatus = normalizedLifecycle
	return version
}

func buildPluginVersionResponse(version models.PluginVersion, artifactRoot string) pluginVersionResponse {
	return pluginVersionResponse{
		PluginVersion:      version,
		PackagePathDisplay: buildPluginPackagePathDisplay(version.PackagePath, artifactRoot),
	}
}

func buildPluginVersionResponsesWithActiveLifecycle(
	versions []models.PluginVersion,
	activeLifecycle string,
	artifactRoot string,
) []pluginVersionResponse {
	if len(versions) == 0 {
		return []pluginVersionResponse{}
	}
	out := make([]pluginVersionResponse, 0, len(versions))
	for _, version := range versions {
		out = append(
			out,
			buildPluginVersionResponse(
				normalizeActivePluginVersionLifecycle(version, activeLifecycle),
				artifactRoot,
			),
		)
	}
	return out
}

func buildPluginVersionResponses(versions []models.PluginVersion, artifactRoot string) []pluginVersionResponse {
	return buildPluginVersionResponsesWithActiveLifecycle(versions, "", artifactRoot)
}

func buildPluginPackagePathDisplay(rawPath string, artifactRoot string) string {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return ""
	}

	cleanedPath := filepath.Clean(filepath.FromSlash(trimmedPath))
	if !filepath.IsAbs(cleanedPath) {
		return filepath.ToSlash(cleanedPath)
	}

	trimmedRoot := strings.TrimSpace(artifactRoot)
	if trimmedRoot != "" {
		cleanedRoot := filepath.Clean(filepath.FromSlash(trimmedRoot))
		if absRoot, err := filepath.Abs(cleanedRoot); err == nil {
			cleanedRoot = absRoot
		}
		if isPathWithinRoot(cleanedRoot, cleanedPath) {
			if relPath, err := filepath.Rel(cleanedRoot, cleanedPath); err == nil {
				relPath = filepath.Clean(relPath)
				if relPath != "." && strings.TrimSpace(relPath) != "" {
					return filepath.ToSlash(relPath)
				}
			}
		}
	}

	baseName := strings.TrimSpace(filepath.Base(cleanedPath))
	if baseName == "" || baseName == "." {
		return filepath.ToSlash(cleanedPath)
	}
	return filepath.ToSlash(filepath.Join("...", baseName))
}

func buildPluginAddressDisplay(runtime string, rawAddress string, artifactRoot string) string {
	trimmedAddress := strings.TrimSpace(rawAddress)
	if trimmedAddress == "" {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(runtime), service.PluginRuntimeJSWorker) {
		return trimmedAddress
	}

	cleanedAddress := filepath.Clean(filepath.FromSlash(trimmedAddress))
	if !filepath.IsAbs(cleanedAddress) {
		return filepath.ToSlash(cleanedAddress)
	}
	return buildPluginPackagePathDisplay(cleanedAddress, artifactRoot)
}
