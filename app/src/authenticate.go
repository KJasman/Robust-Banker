package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	// jwt/v5 provides JWT implementation with support for signing, verifying and validating tokens
	"github.com/golang-jwt/jwt/v5"
)

// secretKey is used to sign and verify JWT tokens
// This should be stored securely in environment variables
// The key should be at least 32 bytes long for HS256 signing (most secure)
var secretKey = []byte("p2s5v8y/B?E(H+MbQeThWmZq4t7w!z%C")

// Credentials represents the login request payload structure
// json tags ensure proper encoding/decoding of JSON data
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Claims extends jwt.RegisteredClaims to include custom claims,
// adding a Username field to JWT claims
type Claims struct {
	Username             string `json:"username"`
	jwt.RegisteredClaims        // Embedded type provides standard JWT claims (exp, iat, etc.)
}

// Response defines the structure for all API responses
// omitempty tags ensure fields are omitted from JSON if empty
type Response struct {
	Message string `json:"message,omitempty"` // Success messages
	Token   string `json:"token,omitempty"`   // JWT token
	Error   string `json:"error,omitempty"`   // Error messages
}

// loginHandler processes authentication requests and generates JWT tokens
// Accepts POST requests with JSON payload containing username + password
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure endpoint only accepts POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and decode the JSON request body into Credentials struct
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		sendJSONResponse(w, http.StatusBadRequest, Response{Error: "Invalid request payload"})
		return
	}

	// Validate credentials
	// normally validate against a databas. Also password hashing step
	if creds.Username != "admin" || creds.Password != "password" {
		sendJSONResponse(w, http.StatusUnauthorized, Response{Error: "Invalid credentials"})
		return
	}

	// Create JWT claims with username and expiration time
	claims := &Claims{
		Username: creds.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			// Token expires in 1 hour (standard)
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			// Record when the token was issued
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	// Create a new token with the claims and sign it with HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign the token with our secret key
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, Response{Error: "Error generating token"})
		return
	}

	// Return the signed token to the client
	sendJSONResponse(w, http.StatusOK, Response{
		Message: "authentication successful",
		Token:   tokenString,
	})
}

// authenticateMiddleware wraps HTTP handlers to verify JWT tokens
// Returns a new handler function that includes token validation
func authenticateMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendJSONResponse(w, http.StatusUnauthorized, Response{Error: "Authorization header missing"})
			return
		}

		// Extract token from "Bearer <token>" format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			sendJSONResponse(w, http.StatusUnauthorized, Response{Error: "Invalid authorization"})
			return
		}
		tokenStr := tokenParts[1]

		// Initialize Claims struct to store parsed token claims
		claims := &Claims{}

		// Parse and validate the token
		// The provided function returns the key for validating the token's signature
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return secretKey, nil
		})

		// Check if token is valid and hasn't expired
		if err != nil || !token.Valid {
			sendJSONResponse(w, http.StatusUnauthorized, Response{Error: "Invalid Token"})
			return
		}

		// If token is valid, proceed to the wrapped handler
		next.ServeHTTP(w, r)
	}
}

// protectedHandler handles requests to protected resources
// Only accessible with a valid JWT token
func protectedHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure endpoint only accepts GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return success message for authenticated users
	sendJSONResponse(w, http.StatusOK, Response{
		Message: "Access granted to protected content",
	})
}

// sendJSONResponse is a helper function to send JSON responses
// Handles setting proper headers and encoding response data
func sendJSONResponse(w http.ResponseWriter, statusCode int, response Response) {
	// Set Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Set HTTP status code
	w.WriteHeader(statusCode)
	// Encode and write the response as JSON
	json.NewEncoder(w).Encode(response)
}
