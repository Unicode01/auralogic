package registryruntime

import (
	"context"

	"auralogic/market_registry/pkg/runtimeconfig"
)

type CLI struct {
	channel string
	shared  *sharedRuntime
}

func NewCLI(cfg runtimeconfig.CLI) (*CLI, error) {
	shared, err := newSharedRuntime(cfg.Shared)
	if err != nil {
		return nil, err
	}
	return &CLI{
		channel: cfg.Channel,
		shared:  shared,
	}, nil
}

func (c *CLI) GenerateKeyPair(keyID string) (KeyPairResult, error) {
	return c.shared.GenerateKeyPair(keyID)
}

func (c *CLI) Publish(ctx context.Context, req PublishRequest) error {
	if req.Channel == "" {
		req.Channel = c.channel
	}
	return c.shared.Publish(ctx, req)
}

func (c *CLI) SyncGitHubRelease(ctx context.Context, req GitHubReleaseSyncRequest) (GitHubReleaseSyncResult, error) {
	if req.Channel == "" {
		req.Channel = c.channel
	}
	return c.shared.SyncGitHubRelease(ctx, req)
}

func (c *CLI) RebuildRegistry(ctx context.Context) (RebuildResult, error) {
	return c.shared.RebuildRegistry(ctx)
}
