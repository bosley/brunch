package server

import (
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) createUser(username, password string) (int, error) {
	if err := validateNewUsername(username); err != nil {
		return http.StatusBadRequest, err
	}
	hash, err := getPasswordHash(password)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := s.kvs.CreateUser(username, string(hash)); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusCreated, nil
}

func (s *Server) updateUser(username, password string) (int, error) {
	if err := validateNewUsername(username); err != nil {
		return http.StatusBadRequest, err
	}
	hash, err := getPasswordHash(password)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := s.kvs.UpdateUser(username, string(hash)); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func (s *Server) deleteUser(username, password string) (int, error) {
	if len(password) > 0 {
		s.logger.Info("password given to delete with no effect", "len", len(password))
	}
	if err := s.kvs.DeleteUser(username); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func getPasswordHash(password string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func validateNewUsername(username string) error {
	if len(username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return errors.New("username can only contain a-z, A-Z, 0-9, hyphen, or underscore")
		}
	}
	return nil
}

func (s *Server) authenticateUsernamePassword(username, password string) (bool, error) {
	u, e := s.kvs.GetUser(username)
	if e != nil {
		return false, errors.New("unknown user")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return false, errors.New("invalid password")
	}
	return true, nil
}
