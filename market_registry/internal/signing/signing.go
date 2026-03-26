package signing

import pubsigning "auralogic/market_registry/pkg/signing"

type KeyPair = pubsigning.KeyPair
type Service = pubsigning.Service

func NewService(keyDir string) *Service {
	return pubsigning.NewService(keyDir)
}
