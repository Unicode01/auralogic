package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"auralogic/internal/models"
)

const (
	PluginHostManifestVersion        = "1.0.0"
	DefaultPluginHostProtocolVersion = "1.0.0"
)

type PluginProtocolCompatibilityInspection struct {
	ManifestPresent        bool   `json:"manifest_present"`
	Runtime                string `json:"runtime,omitempty"`
	HostManifestVersion    string `json:"host_manifest_version"`
	ManifestVersion        string `json:"manifest_version,omitempty"`
	HostProtocolVersion    string `json:"host_protocol_version"`
	ProtocolVersion        string `json:"protocol_version,omitempty"`
	MinHostProtocolVersion string `json:"min_host_protocol_version,omitempty"`
	MaxHostProtocolVersion string `json:"max_host_protocol_version,omitempty"`
	Compatible             bool   `json:"compatible"`
	LegacyDefaultsApplied  bool   `json:"legacy_defaults_applied"`
	ReasonCode             string `json:"reason_code,omitempty"`
	Reason                 string `json:"reason,omitempty"`
}

type pluginProtocolManifestMetadata struct {
	Runtime                string `json:"runtime"`
	ManifestVersion        string `json:"manifest_version"`
	ProtocolVersion        string `json:"protocol_version"`
	MinHostProtocolVersion string `json:"min_host_protocol_version"`
	MaxHostProtocolVersion string `json:"max_host_protocol_version"`
}

type pluginCompatVersion struct {
	major int
	minor int
	patch int
}

func hostProtocolVersionByRuntime(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case PluginRuntimeGRPC:
		return "1.0.0"
	case PluginRuntimeJSWorker:
		return "1.0.0"
	default:
		return DefaultPluginHostProtocolVersion
	}
}

func InspectPluginProtocolCompatibility(plugin *models.Plugin) PluginProtocolCompatibilityInspection {
	if plugin == nil {
		return PluginProtocolCompatibilityInspection{
			HostManifestVersion: PluginHostManifestVersion,
			HostProtocolVersion: DefaultPluginHostProtocolVersion,
			Compatible:          false,
			ReasonCode:          "plugin_missing",
			Reason:              "plugin record is unavailable",
		}
	}
	return InspectPluginManifestCompatibility(plugin.Manifest, plugin.Runtime)
}

func InspectPluginManifestCompatibility(rawManifest string, runtime string) PluginProtocolCompatibilityInspection {
	trimmedManifest := strings.TrimSpace(rawManifest)
	if trimmedManifest == "" {
		return InspectPluginManifestCompatibilityMetadata(runtime, "", "", "", "", false)
	}

	var manifest pluginProtocolManifestMetadata
	if err := json.Unmarshal([]byte(trimmedManifest), &manifest); err != nil {
		return PluginProtocolCompatibilityInspection{
			ManifestPresent:     true,
			Runtime:             strings.ToLower(strings.TrimSpace(runtime)),
			HostManifestVersion: PluginHostManifestVersion,
			HostProtocolVersion: hostProtocolVersionByRuntime(runtime),
			Compatible:          false,
			ReasonCode:          "invalid_manifest_json",
			Reason:              fmt.Sprintf("plugin manifest is invalid JSON: %v", err),
		}
	}

	effectiveRuntime := strings.TrimSpace(runtime)
	if effectiveRuntime == "" {
		effectiveRuntime = strings.TrimSpace(manifest.Runtime)
	}
	return InspectPluginManifestCompatibilityMetadata(
		effectiveRuntime,
		manifest.ManifestVersion,
		manifest.ProtocolVersion,
		manifest.MinHostProtocolVersion,
		manifest.MaxHostProtocolVersion,
		true,
	)
}

func InspectPluginManifestCompatibilityMetadata(
	runtime string,
	manifestVersion string,
	protocolVersion string,
	minHostProtocolVersion string,
	maxHostProtocolVersion string,
	manifestPresent bool,
) PluginProtocolCompatibilityInspection {
	normalizedRuntime := strings.ToLower(strings.TrimSpace(runtime))
	hostManifestVersion := mustParsePluginCompatVersion(PluginHostManifestVersion).String()
	hostProtocolVersion := mustParsePluginCompatVersion(hostProtocolVersionByRuntime(normalizedRuntime)).String()

	inspection := PluginProtocolCompatibilityInspection{
		ManifestPresent:     manifestPresent,
		Runtime:             normalizedRuntime,
		HostManifestVersion: hostManifestVersion,
		HostProtocolVersion: hostProtocolVersion,
		Compatible:          true,
	}
	if !manifestPresent {
		inspection.LegacyDefaultsApplied = true
		inspection.ManifestVersion = hostManifestVersion
		inspection.ProtocolVersion = hostProtocolVersion
		inspection.ReasonCode = "manifest_missing_assumed_legacy"
		inspection.Reason = "plugin manifest is missing, so host compatibility defaults are assumed"
		return inspection
	}

	hostManifest, _ := parsePluginCompatVersion(PluginHostManifestVersion)
	hostProtocol, _ := parsePluginCompatVersion(hostProtocolVersionByRuntime(normalizedRuntime))

	manifestVersionRaw := strings.TrimSpace(manifestVersion)
	if manifestVersionRaw == "" {
		inspection.LegacyDefaultsApplied = true
		manifestVersionRaw = PluginHostManifestVersion
	}
	manifestCompatVersion, err := parsePluginCompatVersion(manifestVersionRaw)
	if err != nil {
		inspection.Compatible = false
		inspection.ReasonCode = "invalid_manifest_version"
		inspection.Reason = fmt.Sprintf("manifest_version must be a numeric semantic version: %v", err)
		return inspection
	}
	inspection.ManifestVersion = manifestCompatVersion.String()
	if manifestCompatVersion.major != hostManifest.major || manifestCompatVersion.Compare(hostManifest) > 0 {
		inspection.Compatible = false
		inspection.ReasonCode = "manifest_version_unsupported"
		inspection.Reason = fmt.Sprintf(
			"manifest_version %s is not supported by host manifest schema %s",
			inspection.ManifestVersion,
			hostManifest.String(),
		)
		return inspection
	}

	protocolVersionRaw := strings.TrimSpace(protocolVersion)
	if protocolVersionRaw == "" {
		inspection.LegacyDefaultsApplied = true
		protocolVersionRaw = hostProtocol.String()
	}
	protocolCompatVersion, err := parsePluginCompatVersion(protocolVersionRaw)
	if err != nil {
		inspection.Compatible = false
		inspection.ReasonCode = "invalid_protocol_version"
		inspection.Reason = fmt.Sprintf("protocol_version must be a numeric semantic version: %v", err)
		return inspection
	}
	inspection.ProtocolVersion = protocolCompatVersion.String()
	if protocolCompatVersion.major != hostProtocol.major || protocolCompatVersion.Compare(hostProtocol) > 0 {
		inspection.Compatible = false
		inspection.ReasonCode = "protocol_version_unsupported"
		inspection.Reason = fmt.Sprintf(
			"plugin protocol_version %s is newer than host protocol %s",
			inspection.ProtocolVersion,
			hostProtocol.String(),
		)
		return inspection
	}

	if strings.TrimSpace(minHostProtocolVersion) != "" {
		minHostCompatVersion, minErr := parsePluginCompatVersion(minHostProtocolVersion)
		if minErr != nil {
			inspection.Compatible = false
			inspection.ReasonCode = "invalid_min_host_protocol_version"
			inspection.Reason = fmt.Sprintf("min_host_protocol_version must be a numeric semantic version: %v", minErr)
			return inspection
		}
		inspection.MinHostProtocolVersion = minHostCompatVersion.String()
		if hostProtocol.Compare(minHostCompatVersion) < 0 {
			inspection.Compatible = false
			inspection.ReasonCode = "host_protocol_too_old"
			inspection.Reason = fmt.Sprintf(
				"host protocol %s is older than required minimum %s",
				hostProtocol.String(),
				minHostCompatVersion.String(),
			)
			return inspection
		}
	}

	if strings.TrimSpace(maxHostProtocolVersion) != "" {
		maxHostCompatVersion, maxErr := parsePluginCompatVersion(maxHostProtocolVersion)
		if maxErr != nil {
			inspection.Compatible = false
			inspection.ReasonCode = "invalid_max_host_protocol_version"
			inspection.Reason = fmt.Sprintf("max_host_protocol_version must be a numeric semantic version: %v", maxErr)
			return inspection
		}
		inspection.MaxHostProtocolVersion = maxHostCompatVersion.String()
		if hostProtocol.Compare(maxHostCompatVersion) > 0 {
			inspection.Compatible = false
			inspection.ReasonCode = "host_protocol_too_new"
			inspection.Reason = fmt.Sprintf(
				"host protocol %s is newer than declared maximum %s",
				hostProtocol.String(),
				maxHostCompatVersion.String(),
			)
			return inspection
		}
	}

	if inspection.MinHostProtocolVersion != "" && inspection.MaxHostProtocolVersion != "" {
		minHostCompatVersion, _ := parsePluginCompatVersion(inspection.MinHostProtocolVersion)
		maxHostCompatVersion, _ := parsePluginCompatVersion(inspection.MaxHostProtocolVersion)
		if minHostCompatVersion.Compare(maxHostCompatVersion) > 0 {
			inspection.Compatible = false
			inspection.ReasonCode = "invalid_host_protocol_range"
			inspection.Reason = fmt.Sprintf(
				"min_host_protocol_version %s cannot be greater than max_host_protocol_version %s",
				minHostCompatVersion.String(),
				maxHostCompatVersion.String(),
			)
			return inspection
		}
	}

	if inspection.LegacyDefaultsApplied {
		inspection.ReasonCode = "compatible_assumed_legacy"
		inspection.Reason = "plugin uses compatibility defaults because manifest version metadata is incomplete"
		return inspection
	}

	inspection.ReasonCode = "compatible"
	inspection.Reason = "plugin manifest compatibility metadata matches the current host"
	return inspection
}

func ValidatePluginManifestCompatibility(rawManifest string, runtime string) error {
	inspection := InspectPluginManifestCompatibility(rawManifest, runtime)
	if inspection.Compatible {
		return nil
	}
	return errors.New(inspection.Reason)
}

func ValidatePluginProtocolCompatibility(plugin *models.Plugin) error {
	inspection := InspectPluginProtocolCompatibility(plugin)
	if inspection.Compatible {
		return nil
	}
	return errors.New(inspection.Reason)
}

func parsePluginCompatVersion(raw string) (pluginCompatVersion, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return pluginCompatVersion{}, fmt.Errorf("version is required")
	}
	trimmed = strings.TrimPrefix(strings.ToLower(trimmed), "v")

	parts := strings.Split(trimmed, ".")
	if len(parts) > 3 {
		return pluginCompatVersion{}, fmt.Errorf("version %q has too many segments", raw)
	}

	values := [3]int{}
	for idx, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return pluginCompatVersion{}, fmt.Errorf("version %q contains an empty segment", raw)
		}
		value := 0
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return pluginCompatVersion{}, fmt.Errorf("version %q contains non-numeric segment %q", raw, part)
			}
			value = value*10 + int(ch-'0')
		}
		values[idx] = value
	}

	return pluginCompatVersion{
		major: values[0],
		minor: values[1],
		patch: values[2],
	}, nil
}

func mustParsePluginCompatVersion(raw string) pluginCompatVersion {
	parsed, err := parsePluginCompatVersion(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func (v pluginCompatVersion) Compare(other pluginCompatVersion) int {
	switch {
	case v.major < other.major:
		return -1
	case v.major > other.major:
		return 1
	case v.minor < other.minor:
		return -1
	case v.minor > other.minor:
		return 1
	case v.patch < other.patch:
		return -1
	case v.patch > other.patch:
		return 1
	default:
		return 0
	}
}

func (v pluginCompatVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}
