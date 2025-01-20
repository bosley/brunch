/*
Environment Variables Required:

	BRUNCH_JWT_SECRET
	BRUNCH_SECRET_KEY

Optional:

	BRUNCH_BINDING 		(defaults to ApplicationBinding [below])
	BRUNCH_KEY_PATH		If both key and cert are valid, then the server will serve over HTTPS
	BRUNCH_CERT_PATH	if one or the other is set or something weird is UB
	BRUNCH_DATASTORE	(defaults to ApplicationDatastore [below])
*/
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bosley/brunch/internal/server"
)

const (
	ApplicationBinding       = "localhost:9764"
	ApplicationDatastoreName = "brunch.db"
)

var (
	Binding              = ApplicationBinding
	JWTSecret            = ""
	SecretKey            = ""
	ApplicationDatastore = ApplicationDatastoreName
)

var tlsInfo *server.Https

func init() {
	JWTSecret = os.Getenv("BRUNCH_JWT_SECRET")
	SecretKey = os.Getenv("BRUNCH_SECRET_KEY")

	if JWTSecret == "" {
		s, e := GenerateSecret()
		if e != nil {
			fmt.Println("Failed to generate JWT secret, and BRUNCH_JWT_SECRET not set", e)
			os.Exit(1)
		}
		JWTSecret = s
	}

	if SecretKey == "" {
		s, e := GenerateSecret()
		if e != nil {
			fmt.Println("Failed to generate BRUNCH_SECRET_KEY secret, and BRUNCH_SECRET_KEY not set", e)
			os.Exit(1)
		}
		fmt.Println("WARNING: You did not set the secret key via BRUNCH_SECRET_KEY so one was generated")
		fmt.Println("You will need it to store this in your ENV as BRUNCH_SECRET_KEY to retrieve your data")
		SecretKey = s
	}

	optBinding := os.Getenv("BRUNCH_BINDING")
	if optBinding != "" {
		Binding = optBinding
	}

	optDatastore := os.Getenv("BRUNCH_DATASTORE")
	if optDatastore != "" {
		ApplicationDatastore = optDatastore
	}

	keyPath := os.Getenv("BRUNCH_KEY_PATH")
	certPath := os.Getenv("BRUNCH_CERT_PATH")
	if keyPath != "" && certPath != "" {
		tlsInfo = &server.Https{
			KeyPath:  keyPath,
			CertPath: certPath,
		}
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	s, err := server.New(server.Opts{
		Binding:       Binding,
		JWTSecret:     JWTSecret,
		SecretKey:     SecretKey,
		Logger:        logger,
		TLSPaths:      tlsInfo,
		DataStorePath: ApplicationDatastore,
	})

	if err != nil {
		fmt.Println("Failed to create server:", err)
		os.Exit(1)
	}

	if err := s.ServeForever(); err != nil {
		fmt.Println("Failed to serve:", err)
		os.Exit(1)
	}
}
