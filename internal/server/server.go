package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bosley/brunch"
	"github.com/bosley/brunch/api"
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"github.com/golang-jwt/jwt/v5"
)

type Server struct {
	fServer   *fuego.Server
	provider  brunch.Provider
	jwtSecret string
	secretKey string
	logger    *slog.Logger
	tlsPaths  *Https
	kvs       *KVS
}

type Https struct {
	KeyPath  string
	CertPath string
}

type Opts struct {
	Binding       string
	Provider      brunch.Provider
	JWTSecret     string
	SecretKey     string
	Logger        *slog.Logger
	TLSPaths      *Https
	DataStorePath string
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func New(opts Opts) (*Server, error) {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}))
	}

	s := &Server{
		provider:  opts.Provider,
		jwtSecret: opts.JWTSecret,
		secretKey: opts.SecretKey,
		logger:    opts.Logger.WithGroup("brunch"),
		tlsPaths:  opts.TLSPaths,
		fServer: fuego.NewServer(
			fuego.WithAddr(opts.Binding),
		),
	}

	fuego.Post(s.fServer, "/api/v1/auth", s.handleAuth,
		option.Summary("Authentication endpoint"),
		option.Description("Authenticate users and provide JWT tokens"),
	)

	fuego.Post(s.fServer, "/api/v1/brunch", s.handleQuery,
		option.Summary("Query endpoint"),
		option.Description("Query the server"),
	)

	var err error
	s.kvs, err = NewKVS(opts.DataStorePath)
	return s, err
}

func (s *Server) generateToken(username string) (string, error) {
	expirationTime := time.Now().Add(12 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Server) validateToken(tokenStr string) (*Claims, error) {

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (s *Server) handleAuth(c fuego.ContextWithBody[api.BrunchAuthRequest]) (api.BrunchAuthResponse, error) {
	response := api.BrunchAuthResponse{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
		Token:   "",
	}
	b, err := c.Body()
	if err != nil {
		return response, err
	}
	ok, err := s.authenticateUsernamePassword(b.Username, b.Password)
	if err != nil {
		response.Code = http.StatusInternalServerError
		response.Message = "Error authenticating"
		return response, errors.New("error authenticating")
	}
	if !ok {
		response.Code = http.StatusUnauthorized
		response.Message = "Unauthorized - invalid credentials"
		return response, errors.New("invalid credentials")
	}
	token, err := s.generateToken(b.Username)
	if err != nil {
		response.Code = http.StatusInternalServerError
		response.Message = "Error generating token"
		return response, err
	}
	response.Code = http.StatusOK
	response.Message = "Authorized"
	response.Token = token
	return response, nil
}

func (s *Server) handleQuery(c fuego.ContextWithBody[api.BrunchQueryRequest]) (api.BrunchQueryResponse, error) {
	response := api.BrunchQueryResponse{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
		Result:  "",
	}
	b, err := c.Body()
	if err != nil {
		response.Message = "Error parsing request"
		response.Code = http.StatusBadRequest
		return response, err
	}
	_, err = s.validateToken(b.Token)
	if err != nil {
		response.Code = http.StatusUnauthorized
		response.Message = "Invalid token"
		return response, err
	}
	return s.executeQuery(b.Query)
}

func (s *Server) ServeForever() error {

	if s.tlsPaths != nil {
		return s.fServer.RunTLS(s.tlsPaths.CertPath, s.tlsPaths.KeyPath)
	}
	return s.fServer.Run()
}

func (s *Server) handleAdminRequest(c fuego.ContextWithBody[api.BrunchAdminRequest]) (api.BrunchAdminResponse, error) {
	response := api.BrunchAdminResponse{
		Code: http.StatusUnauthorized,
	}
	b, err := c.Body()
	if err != nil {
		response.Code = http.StatusBadRequest
		return response, err
	}
	if b.SecretKey != s.secretKey {
		response.Code = http.StatusUnauthorized
		return response, nil
	}

	var opErr error
	switch b.Op {
	case api.BranchOpCreate:
		response.Code, opErr = s.createUser(b.Username, b.Password)
	case api.BranchOpUpdate:
		response.Code, opErr = s.updateUser(b.Username, b.Password)
	case api.BranchOpDelete:
		response.Code, opErr = s.deleteUser(b.Username, b.Password)
	default:
		response.Code = http.StatusBadRequest
		opErr = errors.New("invalid operation")
	}
	return response, opErr
}
