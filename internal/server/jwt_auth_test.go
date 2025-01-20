package server

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestJWTTokenGeneration(t *testing.T) {
	// Setup test environment
	jwtSecret := "test-jwt-secret"
	secretKey := "test-secret-key"
	username := "jwtuser"

	kvs, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := &Server{
		jwtSecret: jwtSecret,
		secretKey: secretKey,
		kvs:       kvs,
	}

	// Test token generation
	t.Run("Generate Valid Token", func(t *testing.T) {
		token, err := s.generateToken(username)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		if token == "" {
			t.Error("Generated token is empty")
		}

		// Parse and validate the token
		claims := &Claims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil {
			t.Errorf("Failed to parse token: %v", err)
		}
		if !parsedToken.Valid {
			t.Error("Token is invalid")
		}
		if claims.Username != username {
			t.Errorf("Expected username %s, got %s", username, claims.Username)
		}
	})
}

func TestJWTTokenValidation(t *testing.T) {
	// Setup test environment
	jwtSecret := "test-jwt-secret"
	secretKey := "test-secret-key"
	username := "jwtuser"

	kvs, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := &Server{
		jwtSecret: jwtSecret,
		secretKey: secretKey,
		kvs:       kvs,
	}

	// Generate a valid token for testing
	validToken, err := s.generateToken(username)
	if err != nil {
		t.Fatalf("Failed to generate token for testing: %v", err)
	}

	t.Run("Validate Valid Token", func(t *testing.T) {
		claims, err := s.validateToken(validToken)
		if err != nil {
			t.Errorf("Failed to validate valid token: %v", err)
		}
		if claims.Username != username {
			t.Errorf("Expected username %s, got %s", username, claims.Username)
		}
	})

	t.Run("Validate Invalid Token", func(t *testing.T) {
		// Test with malformed token
		_, err := s.validateToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error with invalid token, got nil")
		}

		// Test with wrong signing method
		wrongToken := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
			Username: username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		wrongSignedToken, _ := wrongToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
		_, err = s.validateToken(wrongSignedToken)
		if err == nil {
			t.Error("Expected error with wrong signing method, got nil")
		}

		// Test with expired token
		expiredClaims := &Claims{
			Username: username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
		}
		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		expiredSignedToken, _ := expiredToken.SignedString([]byte(jwtSecret))
		_, err = s.validateToken(expiredSignedToken)
		if err == nil {
			t.Error("Expected error with expired token, got nil")
		}
	})
}

func TestFullAuthFlow(t *testing.T) {
	// Setup test environment
	jwtSecret := "test-jwt-secret"
	secretKey := "test-secret-key"
	username := "authflowuser"
	password := "authflow123"

	kvs, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := &Server{
		jwtSecret: jwtSecret,
		secretKey: secretKey,
		kvs:       kvs,
	}

	// Create test user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	err = kvs.CreateUser(username, string(hashedPassword))
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("Full Authentication Flow", func(t *testing.T) {
		// Step 1: Authenticate user
		ok, err := s.authenticateUsernamePassword(username, password)
		if err != nil {
			t.Fatalf("Authentication failed: %v", err)
		}
		if !ok {
			t.Fatal("Expected successful authentication")
		}

		// Step 2: Generate token
		token, err := s.generateToken(username)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Step 3: Validate token
		claims, err := s.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		if claims.Username != username {
			t.Errorf("Expected username %s, got %s", username, claims.Username)
		}

		// Verify token expiration is in the future
		if claims.ExpiresAt.Time.Before(time.Now()) {
			t.Error("Token is already expired")
		}
	})
}
