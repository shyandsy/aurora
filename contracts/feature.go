package contracts

type Features interface {
	Name() string
	Setup(app App) error
	Close() error
}
