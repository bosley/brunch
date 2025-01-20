package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/bosley/brunch/api"
	"github.com/bosley/brunch/internal/server"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	ServerBinding       = "localhost:9764"
	ServerDatastoreName = "brunch.db"
)

var (
	Binding              = ServerBinding
	ApplicationDatastore = ServerDatastoreName
	SecretKey            = ""
)

func init() {
	optBinding := os.Getenv("BRUNCH_BINDING")
	if optBinding != "" {
		Binding = optBinding
	}

	optDatastore := os.Getenv("BRUNCH_DATASTORE")
	if optDatastore != "" {
		ApplicationDatastore = optDatastore
	}

	SecretKey = os.Getenv("BRUNCH_SECRET_KEY")

	if SecretKey == "" {
		fmt.Println()
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	var useHttps bool
	var skipVerify bool
	loginUser := flag.String("login", "", "login with an existing user")
	flag.String("new-user", "", "create a new user")
	flag.BoolVar(&useHttps, "use-https", false, "run server with https")
	flag.BoolVar(&skipVerify, "skip-verify", false, "skip certificate verification")
	flag.StringVar(&Binding, "binding", Binding, "server binding")
	flag.StringVar(&ApplicationDatastore, "datastore", ApplicationDatastore, "datastore name")
	flag.Parse()

	if username := flag.Lookup("new-user").Value.String(); username != "" {
		password := make([]byte, 16)
		if _, err := rand.Read(password); err != nil {
			slog.Error("Failed to generate random password", "error", err)
			os.Exit(1)
		}
		strPassword := base64.URLEncoding.EncodeToString(password)
		slog.Info("Generated password for new user", "username", username, "password", strPassword)

		// Create KVS instance
		kvs, err := server.NewKVS(ApplicationDatastore)
		if err != nil {
			slog.Error("Failed to create KVS", "error", err)
			os.Exit(1)
		}
		defer kvs.Close()

		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(strPassword), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			os.Exit(1)
		}

		// Create the user
		if err := kvs.CreateUser(username, string(hashedPassword)); err != nil {
			slog.Error("Failed to create user", "error", err)
			os.Exit(1)
		}

		slog.Info("Successfully created user", "username", username)
		os.Exit(0)
	}

	if *loginUser != "" {
		fmt.Printf("Enter password for %s: ", *loginUser)
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			slog.Error("Failed to read password", "error", err)
			os.Exit(1)
		}
		fmt.Println() // Add newline after password input

		client, err := api.NewWithPassword(
			Binding,
			*loginUser,
			string(password),
			api.Opts{
				Https:      useHttps,
				SkipVerify: skipVerify,
			},
		)
		if err != nil {
			slog.Error("Login failed", "error", err)
			os.Exit(1)
		}

		slog.Info("Successfully logged in", "username", *loginUser)

		session := NewSession(client)
		if err := session.Start(); err != nil {
			slog.Error("Session error", "error", err)
			os.Exit(1)
		}
	}
}
