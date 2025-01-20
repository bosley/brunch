package server

import (
	"testing"

	"github.com/bosley/brunch/api"
	"golang.org/x/crypto/bcrypt"
)

func TestQueryOperations(t *testing.T) {
	// Setup test environment
	jwtSecret := "test-jwt-secret"
	secretKey := "test-secret-key"
	username := "queryuser"
	password := "querypass123"

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

	// Test data
	testKey := "testKey"
	testValue := "testValue"
	updatedValue := "updatedValue"

	t.Run("Create Operation", func(t *testing.T) {
		resp, err := s.executeQuery(username, api.BrunchOpCreate, testKey, testValue)
		if err != nil {
			t.Errorf("Failed to execute create query: %v", err)
		}
		if resp.Code != 201 {
			t.Errorf("Expected status code 201, got %d", resp.Code)
		}
		if resp.Result != testValue {
			t.Errorf("Expected result %s, got %s", testValue, resp.Result)
		}

		// Verify data was stored
		value, err := kvs.GetUserData(username, testKey)
		if err != nil {
			t.Errorf("Failed to get user data: %v", err)
		}
		if value != testValue {
			t.Errorf("Expected stored value %s, got %s", testValue, value)
		}
	})

	t.Run("Update Operation", func(t *testing.T) {
		resp, err := s.executeQuery(username, api.BrunchOpUpdate, testKey, updatedValue)
		if err != nil {
			t.Errorf("Failed to execute update query: %v", err)
		}
		if resp.Code != 200 {
			t.Errorf("Expected status code 200, got %d", resp.Code)
		}
		if resp.Result != updatedValue {
			t.Errorf("Expected result %s, got %s", updatedValue, resp.Result)
		}

		// Verify data was updated
		value, err := kvs.GetUserData(username, testKey)
		if err != nil {
			t.Errorf("Failed to get user data: %v", err)
		}
		if value != updatedValue {
			t.Errorf("Expected stored value %s, got %s", updatedValue, value)
		}
	})

	t.Run("Delete Operation", func(t *testing.T) {
		resp, err := s.executeQuery(username, api.BrunchOpDelete, testKey, "")
		if err != nil {
			t.Errorf("Failed to execute delete query: %v", err)
		}
		if resp.Code != 200 {
			t.Errorf("Expected status code 200, got %d", resp.Code)
		}

		// Verify data was deleted
		_, err = kvs.GetUserData(username, testKey)
		if err == nil {
			t.Error("Expected error getting deleted data, got nil")
		}
	})

	t.Run("Invalid Operation", func(t *testing.T) {
		resp, err := s.executeQuery(username, "invalid", testKey, testValue)
		if err == nil {
			t.Error("Expected error with invalid operation, got nil")
		}
		if resp.Code != 400 {
			t.Errorf("Expected status code 400, got %d", resp.Code)
		}
	})
}
