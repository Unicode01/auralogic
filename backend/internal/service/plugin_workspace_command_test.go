package service

import (
	"testing"

	"auralogic/internal/models"
)

func workspaceCommandByName(commands []PluginWorkspaceCommand, name string) *PluginWorkspaceCommand {
	for idx := range commands {
		if commands[idx].Name == name {
			return &commands[idx]
		}
	}
	return nil
}

func TestResolvePluginWorkspaceCommandsReturnsBuiltinsOnlyWithoutRuntimeCatalog(t *testing.T) {
	plugin := &models.Plugin{
		Runtime: PluginRuntimeJSWorker,
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": []
		}`,
	}

	commands := ResolvePluginWorkspaceCommands(plugin)
	if len(commands) == 0 {
		t.Fatalf("expected builtin workspace commands, got 0")
	}
	if customCommand := workspaceCommandByName(commands, "debugger/catalog"); customCommand != nil {
		t.Fatalf("expected custom workspace command to require runtime catalog, got %+v", customCommand)
	}
	lsCommand := workspaceCommandByName(commands, pluginWorkspaceBuiltinCommandLS)
	if lsCommand == nil || !lsCommand.Builtin {
		t.Fatalf("expected builtin ls command to exist, got %+v", lsCommand)
	}
	if lsCommand.Granted {
		t.Fatalf("expected builtin ls command to be gated by runtime.file_system permission")
	}
}

func TestResolvePluginWorkspaceCommandsForPluginReturnsBuiltinsOnly(t *testing.T) {
	plugin := &models.Plugin{
		Runtime: PluginRuntimeJSWorker,
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": []
		}`,
	}
	service := &PluginManagerService{}

	commands := service.ResolvePluginWorkspaceCommandsForPlugin(plugin)
	if len(commands) == 0 {
		t.Fatalf("expected builtin workspace commands, got 0")
	}
	if customCommand := workspaceCommandByName(commands, "debugger/report"); customCommand != nil {
		t.Fatalf("expected runtime workspace command catalog to stay disabled, got %+v", customCommand)
	}
}
