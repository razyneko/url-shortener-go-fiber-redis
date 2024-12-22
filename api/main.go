package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger" // Correct import for v2
	"github.com/joho/godotenv"
	"github.com/razyneko/url-shortener-go-fiber-redis/routes"
)

func setUpRoutes(app *fiber.App) {
	app.Get("/:url", routes.ResolveURL)
	app.Post("/api/v1", routes.ShortenURL)
}

func main() {
	err := godotenv.Load()

	if err != nil {
		fmt.Println("Error loading .env file", err)
	}

	// Creating instance of fiber app
	app := fiber.New()

	// Correctly using logger middleware
	app.Use(logger.New()) // This should work without any issue

	// Set up the routes
	setUpRoutes(app)

	// Listening on port
	
	log.Fatal(app.Listen(os.Getenv("APP_PORT")))
}
