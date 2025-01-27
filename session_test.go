package brunch

import (
	"strings"
	"testing"
)

func TestSession_Execute(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(t *testing.T, called *bool, args []interface{})
	}{
		{
			name:    "new provider command with all required properties",
			content: `\new-provider "test-provider" :host "test-host" :base-url "http://test.com" :max-tokens 1000 :temperature 0.7 :system-prompt "test prompt"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnNewProvider callback was not called")
				}
				if len(args) != 6 {
					t.Errorf("expected 6 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-provider" {
					t.Errorf("expected name 'test-provider', got %s", name)
				}
				host := args[1].(string)
				host = strings.Trim(host, `"`)
				if host != "test-host" {
					t.Errorf("expected host 'test-host', got %s", host)
				}
				baseUrl := args[2].(string)
				baseUrl = strings.Trim(baseUrl, `"`)
				if baseUrl != "http://test.com" {
					t.Errorf("expected baseUrl 'http://test.com', got %s", baseUrl)
				}
				maxTokens := args[3].(int)
				if maxTokens != 1000 {
					t.Errorf("expected maxTokens 1000, got %d", maxTokens)
				}
				temperature := args[4].(float64)
				if temperature != 0.7 {
					t.Errorf("expected temperature 0.7, got %f", temperature)
				}
				systemPrompt := args[5].(string)
				systemPrompt = strings.Trim(systemPrompt, `"`)
				if systemPrompt != "test prompt" {
					t.Errorf("expected systemPrompt 'test prompt', got %s", systemPrompt)
				}
			},
		},
		{
			name:    "new chat command with required provider",
			content: `\new-chat "test-chat" :provider "test-provider"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnNewChat callback was not called")
				}
				if len(args) != 2 {
					t.Errorf("expected 2 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-chat" {
					t.Errorf("expected name 'test-chat', got %s", name)
				}
				provider := args[1].(string)
				provider = strings.Trim(provider, `"`)
				if provider != "test-provider" {
					t.Errorf("expected provider 'test-provider', got %s", provider)
				}
			},
		},
		{
			name:    "chat command without hash",
			content: `\chat "test-chat"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnLoadChat callback was not called")
				}
				if len(args) != 2 {
					t.Errorf("expected 2 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-chat" {
					t.Errorf("expected name 'test-chat', got %s", name)
				}
				hash := args[1].(*string)
				if hash != nil {
					t.Error("expected nil hash")
				}
			},
		},
		{
			name:    "chat command with hash",
			content: `\chat "test-chat" :hash "123abc"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnLoadChat callback was not called")
				}
				if len(args) != 2 {
					t.Errorf("expected 2 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-chat" {
					t.Errorf("expected name 'test-chat', got %s", name)
				}
				hash := args[1].(*string)
				if hash == nil {
					t.Error("expected non-nil hash")
					return
				}
				hashVal := strings.Trim(*hash, `"`)
				if hashVal != "123abc" {
					t.Errorf("expected hash '123abc', got %s", hashVal)
				}
			},
		},
		{
			name:    "new_provider_missing_required_property",
			content: `\new-provider "test-provider"`,
			wantErr: true,
		},
		{
			name:    "new chat missing provider",
			content: `\new-chat "test-chat"`,
			wantErr: true,
		},
		{
			name:    "chat missing name",
			content: `\chat`,
			wantErr: true,
		},
		{
			name:    "new context command with no properties",
			content: `\new-ctx "test-context"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnNewContext callback was not called")
				}
				if len(args) != 4 {
					t.Errorf("expected 4 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-context" {
					t.Errorf("expected name 'test-context', got %s", name)
				}
				dir := args[1].(*string)
				if dir != nil {
					t.Error("expected nil dir")
				}
				database := args[2].(*string)
				if database != nil {
					t.Error("expected nil database")
				}
				web := args[3].(*string)
				if web != nil {
					t.Error("expected nil web")
				}
			},
		},
		{
			name:    "new context command with all properties",
			content: `\new-ctx "test-context" :dir "/test/dir" :database "test.db" :web "http://test.com"`,
			validate: func(t *testing.T, called *bool, args []interface{}) {
				if !*called {
					t.Error("OnNewContext callback was not called")
				}
				if len(args) != 4 {
					t.Errorf("expected 4 args, got %d", len(args))
				}
				name := args[0].(string)
				name = strings.Trim(name, `"`)
				if name != "test-context" {
					t.Errorf("expected name 'test-context', got %s", name)
				}
				dir := args[1].(*string)
				if dir == nil {
					t.Error("expected non-nil dir")
					return
				}
				dirVal := strings.Trim(*dir, `"`)
				if dirVal != "/test/dir" {
					t.Errorf("expected dir '/test/dir', got %s", dirVal)
				}
				database := args[2].(*string)
				if database == nil {
					t.Error("expected non-nil database")
					return
				}
				dbVal := strings.Trim(*database, `"`)
				if dbVal != "test.db" {
					t.Errorf("expected database 'test.db', got %s", dbVal)
				}
				web := args[3].(*string)
				if web == nil {
					t.Error("expected non-nil web")
					return
				}
				webVal := strings.Trim(*web, `"`)
				if webVal != "http://test.com" {
					t.Errorf("expected web 'http://test.com', got %s", webVal)
				}
			},
		},
		{
			name:    "new context missing name",
			content: `\new-ctx`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new session
			session := &coreSession{}

			// Create statement
			stmt := NewStatement(tt.content)
			if err := stmt.Prepare(); err != nil {
				if !tt.wantErr {
					t.Fatalf("failed to prepare statement: %v", err)
				}
				return
			}

			// Track callback calls
			var (
				newProviderCalled bool
				newChatCalled     bool
				loadChatCalled    bool
				newContextCalled  bool
				callbackArgs      []interface{}
			)

			callbacks := OperationalCallback{
				OnNewProvider: func(name, host, baseUrl string, maxTokens int, temperature float64, systemPrompt string) error {
					newProviderCalled = true
					callbackArgs = []interface{}{name, host, baseUrl, maxTokens, temperature, systemPrompt}
					return nil
				},
				OnNewChat: func(name, provider string) error {
					newChatCalled = true
					callbackArgs = []interface{}{name, provider}
					return nil
				},
				OnLoadChat: func(name string, hash *string) error {
					loadChatCalled = true
					callbackArgs = []interface{}{name, hash}
					return nil
				},
				OnNewContext: func(name string, dir, database, web *string) error {
					newContextCalled = true
					callbackArgs = []interface{}{name, dir, database, web}
					return nil
				},
			}

			// Execute statement
			err := session.execute(stmt, callbacks)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Determine which callback should have been called
			var called *bool
			switch stmt.cmd.keyword {
			case "new-provider":
				called = &newProviderCalled
			case "new-chat":
				called = &newChatCalled
			case "chat":
				called = &loadChatCalled
			case "new-ctx":
				called = &newContextCalled
			}

			// Validate callback and args
			tt.validate(t, called, callbackArgs)
		})
	}
}
