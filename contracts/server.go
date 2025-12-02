package contracts

import "github.com/shyandsy/aurora/route"

type ServerFeature interface {
	Features
	RegisterRoutes(routes []route.Route)
	Start() error
	Wait()
}
