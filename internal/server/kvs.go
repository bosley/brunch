package server

import (
	"encoding/json"
	"fmt"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// KVS represents our key-value store
type KVS struct {
	db   *bolt.DB
	path string
	mu   sync.RWMutex
}

// User represents a user in the system
type User struct {
	Username string            `json:"username"`
	Password string            `json:"password"`
	Data     map[string]string `json:"data"`
}

// NewKVS creates a new KVS instance
func NewKVS(path string) (*KVS, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	kvs := &KVS{
		db:   db,
		path: path,
	}

	// Initialize admin bucket
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("admin"))
		if err != nil {
			return fmt.Errorf("failed to create admin bucket: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize admin bucket: %w", err)
	}

	return kvs, nil
}

// Close closes the database
func (k *KVS) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.db.Close()
}

// CreateUser creates a new user
func (k *KVS) CreateUser(username, password string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.db.Update(func(tx *bolt.Tx) error {
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		// Check if user already exists
		if admin.Get([]byte(username)) != nil {
			return fmt.Errorf("user already exists")
		}

		// Create user bucket
		_, err := tx.CreateBucket([]byte(username))
		if err != nil {
			return fmt.Errorf("failed to create user bucket: %w", err)
		}

		// Store user in admin bucket
		user := User{
			Username: username,
			Password: password,
			Data:     make(map[string]string),
		}

		userData, err := json.Marshal(user)
		if err != nil {
			return fmt.Errorf("failed to marshal user data: %w", err)
		}

		return admin.Put([]byte(username), userData)
	})
}

// GetUser retrieves a user
func (k *KVS) GetUser(username string) (*User, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var user *User
	err := k.db.View(func(tx *bolt.Tx) error {
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		data := admin.Get([]byte(username))
		if data == nil {
			return fmt.Errorf("user not found")
		}

		user = &User{}
		return json.Unmarshal(data, user)
	})

	return user, err
}

// UpdateUser updates a user's password
func (k *KVS) UpdateUser(username, newPassword string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.db.Update(func(tx *bolt.Tx) error {
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		data := admin.Get([]byte(username))
		if data == nil {
			return fmt.Errorf("user not found")
		}

		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user data: %w", err)
		}

		user.Password = newPassword

		userData, err := json.Marshal(user)
		if err != nil {
			return fmt.Errorf("failed to marshal user data: %w", err)
		}

		return admin.Put([]byte(username), userData)
	})
}

// DeleteUser deletes a user
func (k *KVS) DeleteUser(username string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.db.Update(func(tx *bolt.Tx) error {
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		// Delete from admin bucket
		if err := admin.Delete([]byte(username)); err != nil {
			return fmt.Errorf("failed to delete user from admin bucket: %w", err)
		}

		// Delete user bucket
		return tx.DeleteBucket([]byte(username))
	})
}

// SetUserData sets a key-value pair in user's bucket
func (k *KVS) SetUserData(username, key, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.db.Update(func(tx *bolt.Tx) error {
		// Update user's data in their bucket
		b := tx.Bucket([]byte(username))
		if b == nil {
			return fmt.Errorf("user bucket not found")
		}

		if err := b.Put([]byte(key), []byte(value)); err != nil {
			return fmt.Errorf("failed to set user data: %w", err)
		}

		// Update user's data in admin bucket
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		data := admin.Get([]byte(username))
		if data == nil {
			return fmt.Errorf("user not found in admin bucket")
		}

		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user data: %w", err)
		}

		if user.Data == nil {
			user.Data = make(map[string]string)
		}
		user.Data[key] = value

		userData, err := json.Marshal(user)
		if err != nil {
			return fmt.Errorf("failed to marshal user data: %w", err)
		}

		return admin.Put([]byte(username), userData)
	})
}

// GetUserData gets a value from user's bucket
func (k *KVS) GetUserData(username, key string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var value string
	err := k.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(username))
		if b == nil {
			return fmt.Errorf("user bucket not found")
		}

		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key not found")
		}

		value = string(data)
		return nil
	})

	return value, err
}

// DeleteUserData deletes a key-value pair from user's bucket
func (k *KVS) DeleteUserData(username, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.db.Update(func(tx *bolt.Tx) error {
		// Delete from user's bucket
		b := tx.Bucket([]byte(username))
		if b == nil {
			return fmt.Errorf("user bucket not found")
		}

		if err := b.Delete([]byte(key)); err != nil {
			return fmt.Errorf("failed to delete user data: %w", err)
		}

		// Update user's data in admin bucket
		admin := tx.Bucket([]byte("admin"))
		if admin == nil {
			return fmt.Errorf("admin bucket not found")
		}

		data := admin.Get([]byte(username))
		if data == nil {
			return fmt.Errorf("user not found in admin bucket")
		}

		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user data: %w", err)
		}

		delete(user.Data, key)

		userData, err := json.Marshal(user)
		if err != nil {
			return fmt.Errorf("failed to marshal user data: %w", err)
		}

		return admin.Put([]byte(username), userData)
	})
}
