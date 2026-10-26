package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "../data/chicken-road.sqlite"
	}

	app, err := newApp(databasePath, os.Getenv("FRONTEND_ORIGIN"))
	if err != nil {
		log.Fatalf("start chicken road backend: %v", err)
	}
	defer func() {
		if err := app.close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Chicken Road API is listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run chicken road backend: %v", err)
	}
}
