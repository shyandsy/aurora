package app

import (
	"fmt"
	"log"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/route"

	"github.com/shyandsy/di"
)

type appConfig struct {
	Server config.ServerConfig
}

type app struct {
	config        *appConfig
	features      []contracts.Features
	serverFeature contracts.ServerFeature
	di.Container
}

func NewApp() contracts.App {
	cfg := &appConfig{}
	if err := config.ResolveConfig(&cfg.Server); err != nil {
		log.Fatalf("Failed to load app config: %v", err)
	}

	container := di.NewContainer()
	app := &app{
		config:    cfg,
		features:  make([]contracts.Features, 0),
		Container: container,
	}

	app.registerBaseDependencies()

	return app
}

func (a *app) GetContainer() di.Container {
	return a.Container
}

func (a *app) AddFeature(f contracts.Features) {
	if server, ok := f.(contracts.ServerFeature); ok {
		a.serverFeature = server
	}
	if err := f.Setup(a); err != nil {
		log.Fatalf("Failed to setup feature %s: %v", f.Name(), err)
	}
	a.features = append(a.features, f)
}

func (a *app) RegisterRoutes(routes []route.Route) {
	a.serverFeature.RegisterRoutes(routes)
}

func (a *app) registerBaseDependencies() {
	if err := a.Provide(&a.config.Server); err != nil {
		log.Fatalf("Failed to register ServerConfig: %v", err)
	}
}

func (a *app) Run() error {
	a.printStartupInfo()

	if err := a.serverFeature.Start(); err != nil {
		return fmt.Errorf("server start failed: %w", err)
	}

	a.serverFeature.Wait()
	return nil
}

func (a *app) Shutdown() error {
	for i := len(a.features) - 1; i >= 0; i-- {
		if err := a.features[i].Close(); err != nil {
			log.Printf("Error closing feature %s: %v", a.features[i].Name(), err)
		}
	}
	return nil
}

func (a *app) printStartupInfo() {
	fmt.Println("########################################################")
	fmt.Printf("#\t 🏠 Aurora Framework\n")
	fmt.Printf("#\t Service: %s\n", a.config.Server.Name)
	fmt.Printf("#\t Version: %s\n", a.config.Server.Version)
	fmt.Printf("#\t RunLevel: %s\n", a.config.Server.RunLevel)
	fmt.Printf("#\t Address: %s:%d\n", a.config.Server.Host, a.config.Server.Port)
	fmt.Printf("#\t GinMode: %s\n", a.config.Server.GinMode())
	fmt.Println("########################################################")
	fmt.Println()
}
