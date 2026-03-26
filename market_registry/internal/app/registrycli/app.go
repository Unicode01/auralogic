package registrycli

import pubregistrycli "auralogic/market_registry/pkg/registrycli"

type App = pubregistrycli.App

func New() App {
	return pubregistrycli.New()
}
