package runtimeconfig

import pubruntimeconfig "auralogic/market_registry/pkg/runtimeconfig"

const ModulePath = pubruntimeconfig.ModulePath

type Shared = pubruntimeconfig.Shared
type API = pubruntimeconfig.API
type CLI = pubruntimeconfig.CLI
type EnvBinding = pubruntimeconfig.EnvBinding
type CommandBinding = pubruntimeconfig.CommandBinding
type FilesystemBinding = pubruntimeconfig.FilesystemBinding

func LoadShared() Shared {
	return pubruntimeconfig.LoadShared()
}

func LoadAPI() API {
	return pubruntimeconfig.LoadAPI()
}

func LoadCLI() CLI {
	return pubruntimeconfig.LoadCLI()
}

func SharedBindings() []EnvBinding {
	return pubruntimeconfig.SharedBindings()
}

func CLIBindings() []EnvBinding {
	return pubruntimeconfig.CLIBindings()
}

func APIBindings() []EnvBinding {
	return pubruntimeconfig.APIBindings()
}

func CLICommandBinding() CommandBinding {
	return pubruntimeconfig.CLICommandBinding()
}

func APICommandBinding() CommandBinding {
	return pubruntimeconfig.APICommandBinding()
}

func CLIFilesystemBinding() FilesystemBinding {
	return pubruntimeconfig.CLIFilesystemBinding()
}

func APIFilesystemBinding() FilesystemBinding {
	return pubruntimeconfig.APIFilesystemBinding()
}
