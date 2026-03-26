package service

import (
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestBuildManagedWorkerArgsEncodesBoolFlagsInline(t *testing.T) {
	pluginCfg := config.PluginPlatformConfig{
		ArtifactDir:            "data/plugins",
		JSFSMaxFiles:           2048,
		JSFSMaxTotalBytes:      134217728,
		JSFSMaxReadBytes:       4194304,
		JSStorageMaxKeys:       512,
		JSStorageMaxTotalBytes: 4194304,
		JSStorageMaxValueBytes: 65537,
		Sandbox: config.PluginSandboxConfig{
			ExecTimeoutMs:     30000,
			MaxConcurrency:    4,
			MaxMemoryMB:       128,
			JSAllowNetwork:    true,
			JSAllowFileSystem: true,
		},
	}

	args := buildManagedWorkerArgs(pluginCfg, "unix", "/tmp/auralogic-jsworker.sock")
	joined := make(map[string]struct{}, len(args))
	for _, arg := range args {
		joined[arg] = struct{}{}
	}

	if _, ok := joined["-allow-network=true"]; !ok {
		t.Fatalf("expected inline bool flag -allow-network=true, got %#v", args)
	}
	if _, ok := joined["-allow-fs=true"]; !ok {
		t.Fatalf("expected inline bool flag -allow-fs=true, got %#v", args)
	}
	if _, ok := joined["true"]; ok {
		t.Fatalf("unexpected standalone bool value found in args: %#v", args)
	}
	if _, ok := joined["65537"]; !ok {
		t.Fatalf("expected storage max value bytes to remain after bool flags, got %#v", args)
	}
}

func TestToSandboxConfigForActionIncludesStorageProfiles(t *testing.T) {
	supervisor := &JSWorkerSupervisor{}
	plugin := &models.Plugin{
		Name: "template-plugin",
		Capabilities: `{
			"requested_permissions": ["api.execute"],
			"granted_permissions": ["api.execute"],
			"execute_action_storage": {
				"template.page.get": "read",
				"template.page.save": "write",
				"template.echo": "none"
			}
		}`,
	}

	sandboxCfg := supervisor.toSandboxConfigForAction(plugin, "template.page.save", 0)

	if got := sandboxCfg.CurrentAction; got != "template.page.save" {
		t.Fatalf("expected current_action=template.page.save, got %q", got)
	}
	if got := sandboxCfg.DeclaredStorageAccess; got != pluginStorageAccessWrite {
		t.Fatalf("expected declared storage access write, got %q", got)
	}
	if got := sandboxCfg.ExecuteActionStorage["template.page.get"]; got != pluginStorageAccessRead {
		t.Fatalf("expected template.page.get=read, got %#v", sandboxCfg.ExecuteActionStorage)
	}
	if got := sandboxCfg.ExecuteActionStorage["template.echo"]; got != pluginStorageAccessNone {
		t.Fatalf("expected template.echo=none, got %#v", sandboxCfg.ExecuteActionStorage)
	}
}

func TestWorkerEndpointRejectsNonLocalTCPAddress(t *testing.T) {
	supervisor := &JSWorkerSupervisor{
		cfg: &config.Config{
			Plugin: config.PluginPlatformConfig{
				Sandbox: config.PluginSandboxConfig{
					JSWorkerSocketPath: "tcp://0.0.0.0:17345",
				},
			},
		},
	}

	if _, _, err := supervisor.workerEndpoint(); err == nil {
		t.Fatalf("expected non-local js worker tcp endpoint to be rejected")
	}
}
