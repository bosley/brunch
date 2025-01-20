package server

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func setupTestEnvironment(t *testing.T) (*KVS, func()) {
	// Create a temporary database file
	tmpFile, err := os.CreateTemp("", "brunch-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Create new KVS instance
	kvs, err := NewKVS(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		t.Fatalf("Failed to create KVS: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		kvs.Close()
		os.Remove(tmpPath)
	}

	return kvs, cleanup
}

func TestBasicUserCRUD(t *testing.T) {
	kvs, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test data
	username := "testuser"
	password := "testpass123"
	newPassword := "newpass456"

	// Test CREATE
	t.Run("Create User", func(t *testing.T) {
		// Hash the password before storing
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("Failed to hash password: %v", err)
		}

		err = kvs.CreateUser(username, string(hashedPassword))
		if err != nil {
			t.Errorf("Failed to create user: %v", err)
		}

		// Verify user exists
		user, err := kvs.GetUser(username)
		if err != nil {
			t.Errorf("Failed to get created user: %v", err)
		}
		if user.Username != username {
			t.Errorf("Expected username %s, got %s", username, user.Username)
		}

		// Try creating duplicate user
		err = kvs.CreateUser(username, string(hashedPassword))
		if err == nil {
			t.Error("Expected error when creating duplicate user, got nil")
		}
	})

	// Test UPDATE
	t.Run("Update User", func(t *testing.T) {
		// Hash the new password
		hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("Failed to hash new password: %v", err)
		}

		err = kvs.UpdateUser(username, string(hashedNewPassword))
		if err != nil {
			t.Errorf("Failed to update user: %v", err)
		}

		// Verify password was updated
		user, err := kvs.GetUser(username)
		if err != nil {
			t.Errorf("Failed to get updated user: %v", err)
		}

		// Verify the new password hash works
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newPassword))
		if err != nil {
			t.Error("Password was not updated correctly")
		}

		// Try updating non-existent user
		err = kvs.UpdateUser("nonexistent", string(hashedNewPassword))
		if err == nil {
			t.Error("Expected error when updating non-existent user, got nil")
		}
	})

	// Test DELETE
	t.Run("Delete User", func(t *testing.T) {
		err := kvs.DeleteUser(username)
		if err != nil {
			t.Errorf("Failed to delete user: %v", err)
		}

		// Verify user no longer exists
		_, err = kvs.GetUser(username)
		if err == nil {
			t.Error("Expected error when getting deleted user, got nil")
		}

		// Try deleting non-existent user
		err = kvs.DeleteUser(username)
		if err == nil {
			t.Error("Expected error when deleting non-existent user, got nil")
		}
	})
}

func TestUserAuthentication(t *testing.T) {
	// Setup server with required environment
	jwtSecret := "test-jwt-secret"
	secretKey := "test-secret-key"

	kvs, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := &Server{
		jwtSecret: jwtSecret,
		secretKey: secretKey,
		kvs:       kvs,
	}

	// Test data
	username := "authuser"
	password := "authpass123"
	wrongPassword := "wrongpass"

	// Create test user with hashed password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	err = kvs.CreateUser(username, string(hashedPassword))
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Test authentication
	t.Run("User Authentication", func(t *testing.T) {
		// Test valid credentials
		ok, err := s.authenticateUsernamePassword(username, password)
		if err != nil {
			t.Errorf("Authentication failed with valid credentials: %v", err)
		}
		if !ok {
			t.Error("Expected successful authentication, got false")
		}

		// Test invalid password
		ok, err = s.authenticateUsernamePassword(username, wrongPassword)
		if err == nil {
			t.Error("Expected error with invalid password, got nil")
		}
		if ok {
			t.Error("Expected failed authentication, got true")
		}

		// Test non-existent user
		ok, err = s.authenticateUsernamePassword("nonexistent", password)
		if err == nil {
			t.Error("Expected error with non-existent user, got nil")
		}
		if ok {
			t.Error("Expected failed authentication, got true")
		}
	})
}
