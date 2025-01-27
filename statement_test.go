package brunch

import (
	"testing"
)

func TestNewProviderCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid new provider command",
			input:   `\new-provider "my-provider" :host "anthropic" :base-url "some_url" :max-tokens 4096 :temperature 0.7 :system-prompt "prompts/sp-think.xml"`,
			wantErr: false,
		},
		{
			name:    "missing required property",
			input:   `\new-provider "my-provider"`,
			wantErr: true,
		},
		{
			name:    "invalid property type",
			input:   `\new-provider "my-provider" :host 123`,
			wantErr: true,
		},
		{
			name:    "missing command name",
			input:   `\new-provider :host "anthropic"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := NewStatement(tt.input)
			err := stmt.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStatement().Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewChatCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid new chat command",
			input:   `\new-chat "example" :provider "my-provider"`,
			wantErr: false,
		},
		{
			name:    "missing provider property",
			input:   `\new-chat "example"`,
			wantErr: true,
		},
		{
			name:    "missing command name",
			input:   `\new-chat :provider "my-provider"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := NewStatement(tt.input)
			err := stmt.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStatement().Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChatCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid chat command",
			input:   `\chat "example"`,
			wantErr: false,
		},
		{
			name:    "missing command name",
			input:   `\chat`,
			wantErr: true,
		},
		{
			name:    "with optional hash property",
			input:   `\chat "example" :hash "123456"`,
			wantErr: false,
		},
		{
			name:    "with invalid property",
			input:   `\chat "example" :invalid "value"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := NewStatement(tt.input)
			err := stmt.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStatement().Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseProperty(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		propType  propertyType
		wantValue string
		wantErr   bool
	}{
		{
			name:      "valid string property",
			input:     `:host "anthropic"`,
			propType:  PropertyTypeString,
			wantValue: "anthropic",
			wantErr:   false,
		},
		{
			name:      "valid integer property",
			input:     `:max-tokens 4096`,
			propType:  PropertyTypeInteger,
			wantValue: "4096",
			wantErr:   false,
		},
		{
			name:      "valid real property",
			input:     `:temperature 0.7`,
			propType:  PropertyTypeReal,
			wantValue: "0.7",
			wantErr:   false,
		},
		{
			name:      "invalid string property",
			input:     `:host 123`,
			propType:  PropertyTypeString,
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := NewStatement(tt.input)
			required := map[string]propertyType{
				"host":        PropertyTypeString,
				"max-tokens":  PropertyTypeInteger,
				"temperature": PropertyTypeReal,
			}
			prop := stmt.parseProperty(required, nil)

			if tt.wantErr {
				if prop != nil {
					t.Errorf("parseProperty() expected error, got nil")
				}
				return
			}

			if prop == nil {
				t.Errorf("parseProperty() got nil, want non-nil")
				return
			}

			if prop.prop != tt.wantValue {
				t.Errorf("parseProperty() got value = %v, want %v", prop.prop, tt.wantValue)
			}
		})
	}
}

func TestNewContextCommand(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErr        bool
		wantProperties map[string]string
	}{
		{
			name:    "valid new context with directory",
			input:   `\new-ctx "my-context" :dir "/path/to/dir"`,
			wantErr: false,
			wantProperties: map[string]string{
				"dir": "/path/to/dir",
			},
		},
		{
			name:    "valid new context with database",
			input:   `\new-ctx "my-context" :database "mysql://user:pass@localhost:3306/db"`,
			wantErr: false,
			wantProperties: map[string]string{
				"database": "mysql://user:pass@localhost:3306/db",
			},
		},
		{
			name:    "valid new context with web",
			input:   `\new-ctx "my-context" :web "https://example.com"`,
			wantErr: false,
			wantProperties: map[string]string{
				"web": "https://example.com",
			},
		},
		{
			name:    "valid new context with multiple sources",
			input:   `\new-ctx "my-context" :dir "./docs" :web "http://api.example.com" :database "sqlite://data.db"`,
			wantErr: false,
			wantProperties: map[string]string{
				"dir":      "./docs",
				"web":      "http://api.example.com",
				"database": "sqlite://data.db",
			},
		},
		{
			name:    "missing command name",
			input:   `\new-ctx :dir "/path"`,
			wantErr: true,
		},
		{
			name:    "invalid property name",
			input:   `\new-ctx "my-context" :invalid "value"`,
			wantErr: true,
		},
		{
			name:    "missing property value",
			input:   `\new-ctx "my-context" :dir`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := NewStatement(tt.input)
			err := stmt.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStatement().Prepare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if stmt.cmd.keyword != "new-ctx" {
					t.Errorf("Expected command keyword 'new-ctx', got %s", stmt.cmd.keyword)
				}

				// Verify properties are correctly parsed
				if tt.wantProperties != nil {
					for propName, wantValue := range tt.wantProperties {
						prop, exists := stmt.cmd.properties[propName]
						if !exists {
							t.Errorf("Expected property %q not found", propName)
							continue
						}
						if prop.prop != wantValue {
							t.Errorf("Property %q value = %q, want %q", propName, prop.prop, wantValue)
						}
						if prop.typ != PropertyTypeString {
							t.Errorf("Property %q type = %v, want PropertyTypeString", propName, prop.typ)
						}
					}
				}
			}
		})
	}
}
