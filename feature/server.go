package feature

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/route"
)

type serverFeature struct {
	Config   *config.ServerConfig `inject:""`
	Engine   *gin.Engine
	server   *http.Server
	routes   []route.Route
	stopChan chan os.Signal
	running  bool
	mu       sync.Mutex
	wg       sync.WaitGroup
}

func NewServerFeature() contracts.ServerFeature {
	return &serverFeature{
		stopChan: make(chan os.Signal, 1),
		running:  false,
	}
}

func (f *serverFeature) Name() string {
	return "server"
}

func (f *serverFeature) Setup(app contracts.App) error {
	if err := app.Resolve(f); err != nil {
		return fmt.Errorf("failed to resolve ServerFeature dependencies: %w", err)
	}

	if err := f.Config.Validate(); err != nil {
		return fmt.Errorf("server configuration validation failed: %w", err)
	}

	f.Engine = f.createGinEngine()

	if err := app.Provide(f.Engine); err != nil {
		return fmt.Errorf("failed to register gin.Engine: %w", err)
	}

	return nil
}

func (f *serverFeature) RegisterRoutes(routes []route.Route) {
	f.routes = append(f.routes, routes...)
}

func (f *serverFeature) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.running {
		return nil
	}

	f.setupRoutes()
	f.server = f.createServer()
	f.running = true

	f.wg.Add(1)
	go f.startServer()

	f.wg.Add(1)
	go f.signalListener()

	return nil
}

func (f *serverFeature) Close() error {
	f.mu.Lock()
	if !f.running || f.server == nil {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	select {
	case f.stopChan <- syscall.SIGTERM:
		time.Sleep(100 * time.Millisecond)
	default:
		return f.shutdownServer()
	}

	return nil
}

func (f *serverFeature) Wait() {
	f.wg.Wait()
}

func (f *serverFeature) createServer() *http.Server {
	addr := f.Config.Host + ":" + strconv.Itoa(f.Config.Port)

	return &http.Server{
		Addr:         addr,
		Handler:      f.Engine,
		ReadTimeout:  f.Config.ReadTimeout,
		WriteTimeout: f.Config.WriteTimeout,
	}
}

func (f *serverFeature) startServer() {
	defer f.wg.Done()

	if f.server == nil {
		log.Printf("Server not initialized")
		return
	}

	fmt.Println()
	log.Printf("Starting server on %s", f.server.Addr)

	if err := f.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Server start failed: %v", err)
		return
	}
}

func (f *serverFeature) signalListener() {
	defer f.wg.Done()

	signal.Notify(f.stopChan, syscall.SIGINT, syscall.SIGTERM)
	<-f.stopChan
	f.shutdownServer()
}

func (f *serverFeature) shutdownServer() error {
	f.mu.Lock()
	if !f.running || f.server == nil {
		f.mu.Unlock()
		return nil
	}
	server := f.server
	f.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), f.Config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		f.mu.Lock()
		f.running = false
		f.mu.Unlock()
		return err
	}

	f.mu.Lock()
	f.running = false
	f.mu.Unlock()

	return nil
}

func (f *serverFeature) createGinEngine() *gin.Engine {
	mode := f.Config.GinMode()
	if mode != "" {
		gin.SetMode(mode)
	}

	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	if err := f.setupCORS(engine); err != nil {
		log.Fatalf("Failed to setup CORS: %v", err)
	}

	return engine
}

func (f *serverFeature) setupCORS(engine *gin.Engine) error {
	corsCfg := &config.CORSConfig{}
	if err := config.ResolveConfig(corsCfg); err != nil {
		return fmt.Errorf("failed to load CORS config: %w", err)
	}

	if len(corsCfg.AllowedOrigins) == 0 && len(corsCfg.AllowedMethods) == 0 && len(corsCfg.AllowedHeaders) == 0 {
		return nil
	}

	if err := corsCfg.Validate(); err != nil {
		return fmt.Errorf("CORS config validation failed: %w", err)
	}

	corsConfig := cors.Config{
		AllowOrigins:     corsCfg.AllowedOrigins,
		AllowMethods:     corsCfg.AllowedMethods,
		AllowHeaders:     corsCfg.AllowedHeaders,
		AllowCredentials: corsCfg.AllowCredentials,
	}
	engine.Use(cors.New(corsConfig))

	return nil
}

func (f *serverFeature) setupHealthCheck() {
	f.Engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"service":   f.Config.Name,
			"version":   f.Config.Version,
			"timestamp": time.Now().Unix(),
		})
	})

	f.Engine.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ready",
			"service": f.Config.Name,
		})
	})
}

func (f *serverFeature) setupRoutes() {
	f.setupHealthCheck()

	for _, r := range f.routes {
		handler := f.createHandler(r.Handler)

		switch r.Method {
		case "GET":
			f.Engine.GET(r.Path, handler)
		case "POST":
			f.Engine.POST(r.Path, handler)
		case "PUT":
			f.Engine.PUT(r.Path, handler)
		case "DELETE":
			f.Engine.DELETE(r.Path, handler)
		case "PATCH":
			f.Engine.PATCH(r.Path, handler)
		}
	}
}

func (f *serverFeature) createHandler(handler route.CustomizedHandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, bizErr := handler(c)
		if bizErr != nil {
			f.handleError(c, bizErr)
			return
		}
		c.JSON(200, data)
	}
}

func (f *serverFeature) handleError(c *gin.Context, err error) {
	bizErr, ok := err.(bizerr.BizError)
	if ok {
		c.JSON(bizErr.HTTPCode(), gin.H{
			"message": bizErr.Message(),
		})
		return
	}

	systemErr := bizerr.New(http.StatusInternalServerError, err)
	c.JSON(systemErr.HTTPCode(), gin.H{
		"message": systemErr.Message(),
	})
}
