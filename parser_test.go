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
			input:   `\new-provider "my-provider" :host "anthropic"`,
			wantErr: true,
		},
		{
			name:    "invalid property type",
			input:   `\new-provider "my-provider" :host 123`,
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

func TestNewRootCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid new root command",
			input:   `\new-root "some-root-for-task-A" :provider "my-provider"`,
			wantErr: false,
		},
		{
			name:    "missing provider property",
			input:   `\new-root "some-root-for-task-A"`,
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
			input:   `\new-chat "example" :root "some-root-for-task-A"`,
			wantErr: false,
		},
		{
			name:    "missing root property",
			input:   `\new-chat "example"`,
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
			input:   `\chat "example" :restore`,
			wantErr: false,
		},
		{
			name:    "missing restore property",
			input:   `\chat "example"`,
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
			wantValue: `"anthropic"`,
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
			permitted := map[string]propertyType{
				"host":        PropertyTypeString,
				"max-tokens":  PropertyTypeInteger,
				"temperature": PropertyTypeReal,
			}
			prop := stmt.parseProperty(permitted)

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
