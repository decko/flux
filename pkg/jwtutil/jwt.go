// Package jwtutil provides shared JWT validation logic used by both the
// domain auth service and the HTTP middleware layer.
package jwtutil

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ValidateJWTToken parses and validates a JWT token string using the given
// HMAC secret. It verifies the signing method is HS256 and returns the
// token claims on success. Returns an error if the token is invalid,
// expired, or signed with an unexpected method.
func ValidateJWTToken(tokenString string, secret []byte) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
