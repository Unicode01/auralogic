import { definePlugin } from "@auralogic/plugin-sdk";
import { execute, executeStream, workspaceHandlers } from "./lib/debugger";
import { PLUGIN_IDENTITY } from "./lib/constants";
import type { PluginHealthResult } from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute,
  executeStream,
  workspace: workspaceHandlers,
  health(_config: unknown, _sandbox: unknown): PluginHealthResult {
    return {
      healthy: true,
      version: "2.5.0",
      metadata: {
        plugin: PLUGIN_IDENTITY,
        runtime: "goja",
        module_system: "commonjs-require"
      }
    };
  }
});
