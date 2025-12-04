package contracts

import (
	"github.com/shyandsy/di"
)

type App interface {
	AddFeature(feature Features)
	RegisterRoutes(routes []Route)
	Run() error
	Shutdown() error

	Name() string
	RunLevel() string

	GetContainer() di.Container
	di.Container
}
