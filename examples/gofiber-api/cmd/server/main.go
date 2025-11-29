// Package main provides the entry point for the GoFiber API server.
// @title GoFiber API Example
// @version 1.0.0
// @description A sample REST API built with GoFiber framework
// @host localhost:3000
// @BasePath /api/v1
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/user/gofiber-api/internal/config"
	"github.com/user/gofiber-api/internal/handlers"
	"github.com/user/gofiber-api/internal/store"
)

var (
	// Version information set by ldflags
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version information")
	port := flag.Int("port", 0, "Override port from config")
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("GoFiber API %s\n", version)
		fmt.Printf("  Commit: %s\n", commit)
		fmt.Printf("  Built:  %s\n", date)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override port if specified
	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Initialize the in-memory store
	dataStore := store.NewMemoryStore()

	// Initialize handlers
	h := handlers.New(dataStore)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      fmt.Sprintf("GoFiber API %s", version),
		ErrorHandler: handlers.ErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.Server.CORSOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"version": version,
		})
	})

	// API v1 routes
	v1 := app.Group("/api/v1")

	// Users routes
	users := v1.Group("/users")
	users.Get("/", h.ListUsers)
	users.Get("/:id", h.GetUser)
	users.Post("/", h.CreateUser)
	users.Put("/:id", h.UpdateUser)
	users.Delete("/:id", h.DeleteUser)

	// Tasks routes
	tasks := v1.Group("/tasks")
	tasks.Get("/", h.ListTasks)
	tasks.Get("/:id", h.GetTask)
	tasks.Post("/", h.CreateTask)
	tasks.Put("/:id", h.UpdateTask)
	tasks.Delete("/:id", h.DeleteTask)

	// Products routes
	products := v1.Group("/products")
	products.Get("/", h.ListProducts)
	products.Get("/:id", h.GetProduct)
	products.Post("/", h.CreateProduct)
	products.Put("/:id", h.UpdateProduct)
	products.Delete("/:id", h.DeleteProduct)

	// Start server in a goroutine
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	go func() {
		log.Printf("Starting GoFiber API %s on %s", version, addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
