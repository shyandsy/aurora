package contracts

type ServerFeature interface {
	Features
	RegisterRoutes(routes []Route)
	Start() error
	Wait()
}
