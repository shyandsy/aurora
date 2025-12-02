package contracts

import (
	"github.com/shyandsy/aurora/route"
	"github.com/shyandsy/di"
)

type App interface {
	AddFeature(feature Features)
	RegisterRoutes(routes []route.Route)
	Run() error
	Shutdown() error

	GetContainer() di.Container
	di.Container
}
