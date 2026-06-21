package api

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testJWTSecret is a test-only JWT secret that meets the 16-character minimum.
const testJWTSecret = "test-jwt-secret-16!"

// testJWTSecretBytes is the byte slice version of testJWTSecret.
var testJWTSecretBytes = []byte(testJWTSecret)

// testUser is a pre-set user ID and role injected into test tokens.
const (
	testUserID = "test-user-id"
	testRole   = "admin"
)

// generateTestToken creates a signed JWT token for testing protected routes.
func generateTestToken() string {
	claims := jwt.MapClaims{
		"sub":   testUserID,
		"email": "test@example.com",
		"role":  testRole,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecretBytes)
	return tokenStr
}

// testToken is a cached test token generated once at init.
var testToken string

func init() {
	testToken = generateTestToken()
}

// authedRequest creates an HTTP request with a valid JWT Bearer token
// set in the Authorization header, suitable for testing protected routes.
func authedRequest(method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), method, url, body)
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}
