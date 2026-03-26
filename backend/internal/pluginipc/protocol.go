package pluginipc

type SandboxConfig struct {
	Level                   string            `json:"level,omitempty"`
	TimeoutMs               int               `json:"timeout_ms,omitempty"`
	MaxMemoryMB             int               `json:"max_memory_mb,omitempty"`
	MaxConcurrency          int               `json:"max_concurrency,omitempty"`
	CurrentAction           string            `json:"current_action,omitempty"`
	DeclaredStorageAccess   string            `json:"declared_storage_access_mode,omitempty"`
	StorageAccessMode       string            `json:"storage_access_mode,omitempty"`
	AllowNetwork            bool              `json:"allow_network,omitempty"`
	AllowFileSystem         bool              `json:"allow_file_system,omitempty"`
	AllowHookExecute        bool              `json:"allow_hook_execute,omitempty"`
	AllowHookBlock          bool              `json:"allow_hook_block,omitempty"`
	AllowPayloadPatch       bool              `json:"allow_payload_patch,omitempty"`
	AllowFrontendExtensions bool              `json:"allow_frontend_extensions,omitempty"`
	AllowExecuteAPI         bool              `json:"allow_execute_api,omitempty"`
	RequestedPermissions    []string          `json:"requested_permissions,omitempty"`
	GrantedPermissions      []string          `json:"granted_permissions,omitempty"`
	ExecuteActionStorage    map[string]string `json:"execute_action_storage,omitempty"`
	FSMaxFiles              int               `json:"fs_max_files,omitempty"`
	FSMaxTotalBytes         int64             `json:"fs_max_total_bytes,omitempty"`
	FSMaxReadBytes          int64             `json:"fs_max_read_bytes,omitempty"`
	StorageMaxKeys          int               `json:"storage_max_keys,omitempty"`
	StorageMaxTotalBytes    int64             `json:"storage_max_total_bytes,omitempty"`
	StorageMaxValueBytes    int64             `json:"storage_max_value_bytes,omitempty"`
}

type HostAPIConfig struct {
	Network     string `json:"network,omitempty"`
	Address     string `json:"address,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

type HostRequest struct {
	AccessToken string                 `json:"access_token,omitempty"`
	Action      string                 `json:"action"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

type HostResponse struct {
	Success bool                   `json:"success"`
	Status  int                    `json:"status,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type ExecutionContext struct {
	UserID    uint              `json:"user_id,omitempty"`
	OrderID   uint              `json:"order_id,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type WebhookRequest struct {
	Key         string            `json:"key,omitempty"`
	Method      string            `json:"method,omitempty"`
	Path        string            `json:"path,omitempty"`
	QueryString string            `json:"query_string,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	BodyText    string            `json:"body_text,omitempty"`
	BodyBase64  string            `json:"body_base64,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	RemoteAddr  string            `json:"remote_addr,omitempty"`
}

type WorkspaceBufferEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Channel   string            `json:"channel,omitempty"`
	Level     string            `json:"level,omitempty"`
	Message   string            `json:"message,omitempty"`
	Source    string            `json:"source,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type WorkspaceConfig struct {
	Enabled    bool                   `json:"enabled,omitempty"`
	MaxEntries int                    `json:"max_entries,omitempty"`
	History    []WorkspaceBufferEntry `json:"history,omitempty"`
}

type Request struct {
	Type                     string                 `json:"type"`
	PluginID                 uint                   `json:"plugin_id,omitempty"`
	PluginGeneration         uint                   `json:"plugin_generation,omitempty"`
	PluginName               string                 `json:"plugin_name,omitempty"`
	Action                   string                 `json:"action,omitempty"`
	ScriptPath               string                 `json:"script_path,omitempty"`
	Params                   map[string]string      `json:"params,omitempty"`
	RuntimeCode              string                 `json:"runtime_code,omitempty"`
	RuntimeInspectExpression string                 `json:"runtime_inspect_expression,omitempty"`
	RuntimeInspectDepth      int                    `json:"runtime_inspect_depth,omitempty"`
	Storage                  map[string]string      `json:"storage,omitempty"`
	Context                  *ExecutionContext      `json:"context,omitempty"`
	PluginConfig             map[string]interface{} `json:"plugin_config,omitempty"`
	PluginSecrets            map[string]string      `json:"plugin_secrets,omitempty"`
	Webhook                  *WebhookRequest        `json:"webhook,omitempty"`
	HostAPI                  *HostAPIConfig         `json:"host_api,omitempty"`
	Workspace                *WorkspaceConfig       `json:"workspace,omitempty"`
	Sandbox                  SandboxConfig          `json:"sandbox,omitempty"`
}

type Response struct {
	Success          bool                   `json:"success"`
	Healthy          bool                   `json:"healthy,omitempty"`
	Version          string                 `json:"version,omitempty"`
	Data             map[string]interface{} `json:"data,omitempty"`
	Storage          map[string]string      `json:"storage,omitempty"`
	StorageChanged   bool                   `json:"storage_changed,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
	WorkspaceEntries []WorkspaceBufferEntry `json:"workspace_entries,omitempty"`
	WorkspaceCleared bool                   `json:"workspace_cleared,omitempty"`
	IsFinal          bool                   `json:"is_final,omitempty"`
}
