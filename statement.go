package brunch

import "fmt"

type Statement struct {
	content string
	idx     int
	tokens  []token
	cmd     *cmd
}

func (p *Statement) Reset() {
	p.idx = 0
	p.tokens = []token{}
	p.cmd = nil
}

func (p *Statement) IsPrepared() bool {
	return p.cmd != nil
}

func (p *Statement) Prepare() error {

	if p.cmd != nil {
		p.cmd = nil
	}

	if err := p.tokenize(); err != nil {
		return err
	}

	return nil
}

type cmd struct {
	keyword    string
	nameGiven  string
	properties map[string]*property
}

type tokenType int

const (
	TokenTypeNewProviderCmd tokenType = iota
	TokenTypeNewChatCmd
	TokenTypeChatCmd
	TokenTypeNewContextCmd
	TokenTypePropertyTag
	TokenTypeString
	TokenTypeInteger
	TokenTypeReal
	TokenTypeDelChatCmd
	TokenTypeDelContextCmd
	TokenTypeListContextCmd
	TokenTypeListChatCmd
	TokenTypeDescribeContextCmd
	TokenTypeDescribeChatCmd
)

type propertyType int

const (
	PropertyTypeString propertyType = iota
	PropertyTypeInteger
	PropertyTypeReal
)

type token struct {
	pos       int
	tokenType tokenType
	value     string
}

type property struct {
	id   string
	prop string
	typ  propertyType
}

type frame struct {
	t             tokenType
	keyword       string
	requiredProps map[string]propertyType
	optionalProps map[string]propertyType
	singleton     bool
}

var commands = map[string]frame{
	"\\new-provider": {
		t:       TokenTypeNewProviderCmd,
		keyword: "new-provider",
		requiredProps: map[string]propertyType{
			"host": PropertyTypeString,
		},
		optionalProps: map[string]propertyType{
			"base-url":      PropertyTypeString,
			"system-prompt": PropertyTypeString,
			"max-tokens":    PropertyTypeInteger,
			"temperature":   PropertyTypeReal,
		},
	},
	"\\new-chat": {
		t:       TokenTypeNewChatCmd,
		keyword: "new-chat",
		requiredProps: map[string]propertyType{
			"provider": PropertyTypeString,
		},
		optionalProps: map[string]propertyType{},
	},
	"\\chat": {
		t:             TokenTypeChatCmd,
		keyword:       "chat",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{
			"hash": PropertyTypeString,
		},
	},
	"\\new-ctx": {
		t:             TokenTypeNewContextCmd,
		keyword:       "new-ctx",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{
			"dir":      PropertyTypeString,
			"database": PropertyTypeString,
			"web":      PropertyTypeString,
		},
	},
	"\\del-chat": {
		t:             TokenTypeDelChatCmd,
		keyword:       "del-chat",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
	},
	"\\del-ctx": {
		t:             TokenTypeDelContextCmd,
		keyword:       "del-ctx",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
	},
	"\\list-ctx": {
		t:             TokenTypeListContextCmd,
		keyword:       "list-ctx",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
		singleton:     true,
	},
	"\\list-chat": {
		t:             TokenTypeListChatCmd,
		keyword:       "list-chat",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
		singleton:     true,
	},

	"\\describe-ctx": {
		t:             TokenTypeDescribeContextCmd,
		keyword:       "describe-ctx",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
	},
	"\\describe-chat": {
		t:             TokenTypeDescribeChatCmd,
		keyword:       "describe-chat",
		requiredProps: map[string]propertyType{},
		optionalProps: map[string]propertyType{},
	},
}

func NewStatement(content string) *Statement {
	return &Statement{
		content: content,
		idx:     0,
		tokens:  []token{},
		cmd:     nil,
	}
}

func (p *Statement) skipWhitespace() {
	for p.idx < len(p.content) && (p.content[p.idx] == ' ' || p.content[p.idx] == '\t') {
		p.idx++
	}
}

func (p *Statement) tokenize() error {
	for p.idx < len(p.content) {
		p.skipWhitespace()

		if p.idx >= len(p.content) {
			break
		}

		// Now parse properties based on command type
		switch p.content[p.idx] {
		case '\\':
			if p.idx+2 > len(p.content) {
				return fmt.Errorf("invalid token at %d -> %s", p.idx, p.content[p.idx:])
			}
			start := p.idx
			p.idx++

			// Parse command keyword
			for p.idx < len(p.content) && p.content[p.idx] != ' ' {
				p.idx++
			}

			if p.idx == start {
				return fmt.Errorf("invalid token at %d -> %s", p.idx, p.content[p.idx:])
			}

			cmdStr := p.content[start:p.idx]

			cmdFrame, ok := commands[cmdStr]
			if !ok {
				return fmt.Errorf("unknown command: %s", cmdStr)
			}

			p.cmd = &cmd{
				keyword:    cmdFrame.keyword,
				properties: make(map[string]*property),
			}

			p.tokens = append(p.tokens, token{
				pos:       start,
				tokenType: cmdFrame.t,
				value:     cmdStr,
			})

			// These dont take params
			if cmdFrame.singleton {
				return nil
			}

			// Skip whitespace after command
			p.skipWhitespace()

			// Parse command name (must be a quoted string)
			if p.idx >= len(p.content) {
				return fmt.Errorf("missing command name at position %d", p.idx)
			}

			if p.content[p.idx] != '"' {
				return fmt.Errorf("expected command name to start with '\"' at position %d", p.idx)
			}

			nameToken := p.parseString()
			if nameToken == nil {
				return fmt.Errorf("invalid command name at position %d", p.idx)
			}

			p.cmd.nameGiven = nameToken.prop

			return p.parseProperties(cmdFrame.requiredProps, cmdFrame.optionalProps)
		case ':':
			return nil
		default:
			p.idx++
		}
	}
	return nil
}

func (p *Statement) parseProperties(required map[string]propertyType, optional map[string]propertyType) error {
	for p.idx < len(p.content) {
		p.skipWhitespace()

		if p.idx >= len(p.content) {
			break
		}

		if p.content[p.idx] != ':' {
			p.idx++
			continue
		}

		prop := p.parseProperty(required, optional)
		if prop == nil {
			return fmt.Errorf("failed to parse property at position %d", p.idx)
		}

		p.cmd.properties[prop.id] = prop
		p.skipWhitespace()
	}

	// Verify all required properties are present
	for propName := range required {
		if _, exists := p.cmd.properties[propName]; !exists {
			return fmt.Errorf("missing required property: %s", propName)
		}
	}

	return nil
}

func (p *Statement) parseProperty(required map[string]propertyType, optional map[string]propertyType) *property {
	if p.idx >= len(p.content) || p.content[p.idx] != ':' {
		return nil
	}

	p.idx++ // Skip the colon
	p.skipWhitespace()

	// Parse property name
	start := p.idx
	for p.idx < len(p.content) && (isIdentifierChar(p.content[p.idx]) || p.content[p.idx] == '-') {
		p.idx++
	}

	if start == p.idx {
		return nil
	}

	propertyName := p.content[start:p.idx]
	p.skipWhitespace()

	typ, exists := required[propertyName]
	if !exists {
		typ, exists = optional[propertyName]
		if !exists {
			return nil
		}
	}

	var prop *property
	switch typ {
	case PropertyTypeString:
		prop = p.parseString()
	case PropertyTypeInteger:
		prop = p.parseInteger()
	case PropertyTypeReal:
		prop = p.parseReal()
	}

	if prop != nil {
		prop.id = propertyName
	}

	return prop
}

func isIdentifierChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func (p *Statement) parseString() *property {
	if p.idx >= len(p.content) || p.content[p.idx] != '"' {
		return nil
	}

	start := p.idx
	p.idx++ // Skip opening quote

	for p.idx < len(p.content) {
		if p.content[p.idx] == '"' && (p.idx == 0 || p.content[p.idx-1] != '\\') {
			p.idx++ // Skip closing quote
			return &property{
				prop: p.content[start+1 : p.idx-1],
				typ:  PropertyTypeString,
			}
		}
		p.idx++
	}

	return nil // Unterminated string
}

func (p *Statement) parseInteger() *property {
	if p.idx >= len(p.content) {
		return nil
	}

	p.skipWhitespace()
	start := p.idx

	// Handle negative numbers
	if p.idx < len(p.content) && p.content[p.idx] == '-' {
		p.idx++
	}

	// Must have at least one digit
	if p.idx >= len(p.content) || !isDigit(p.content[p.idx]) {
		p.idx = start
		return nil
	}

	// Parse remaining digits
	for p.idx < len(p.content) && isDigit(p.content[p.idx]) {
		p.idx++
	}

	return &property{
		prop: p.content[start:p.idx],
		typ:  PropertyTypeInteger,
	}
}

func (p *Statement) parseReal() *property {
	if p.idx >= len(p.content) {
		return nil
	}

	p.skipWhitespace()
	start := p.idx

	// Handle negative numbers
	if p.idx < len(p.content) && p.content[p.idx] == '-' {
		p.idx++
	}

	hasDigits := false
	for p.idx < len(p.content) && isDigit(p.content[p.idx]) {
		hasDigits = true
		p.idx++
	}

	if p.idx < len(p.content) && p.content[p.idx] == '.' {
		p.idx++
		for p.idx < len(p.content) && isDigit(p.content[p.idx]) {
			hasDigits = true
			p.idx++
		}
	}

	if !hasDigits {
		p.idx = start
		return nil
	}

	return &property{
		prop: p.content[start:p.idx],
		typ:  PropertyTypeReal,
	}
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
