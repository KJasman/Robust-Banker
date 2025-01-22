package main

import (
	"log"
	"net/http"
)

// main initializes the HTTP server and sets up routing
func main() {
	// Register HTTP handlers for each endpoint
	http.HandleFunc("/login", loginHandler)
	// Wrap protected endpoint with authentication middleware.
	// Account information behind protected content.
	http.HandleFunc("/protected", authenticateMiddleware(protectedHandler))

	// Start the server on port 8080
	log.Printf("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
