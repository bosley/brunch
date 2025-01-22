package brunch

import "fmt"

type tokenType int

const (
	TokenTypeNewProviderCmd tokenType = iota
	TokenTypeNewRootCmd
	TokenTypeNewChatCmd
	TokenTypeChatCmd
	TokenTypePropertyTag
	TokenTypeString
	TokenTypeInteger
	TokenTypeReal
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

type Statement interface {
	Prepare() error
}

type cmd struct {
	keyword    string
	properties map[string]*property
}

type stmt struct {
	content string
	idx     int

	tokens []token

	cmd *cmd
}

type property struct {
	id   string
	prop string
	typ  propertyType
}

func NewStatement(content string) *stmt {
	return &stmt{
		content: content,
		idx:     0,
	}
}

func (p *stmt) Prepare() error {

	fmt.Println("prepare")

	if err := p.tokenize(); err != nil {
		return err
	}

	return nil
}

func (p *stmt) skipWhitespace() {
	for p.idx < len(p.content) && (p.content[p.idx] == ' ' || p.content[p.idx] == '\t') {
		p.idx++
	}
}

func (p *stmt) tokenize() error {
	for p.idx < len(p.content) {
		p.skipWhitespace()

		if p.idx >= len(p.content) {
			break
		}

		switch p.content[p.idx] {
		case '\\':
			if p.idx+2 > len(p.content) {
				return fmt.Errorf("invalid token at %d -> %s", p.idx, p.content[p.idx:])
			}
			start := p.idx
			p.idx++

			for p.idx < len(p.content) && p.content[p.idx] != ' ' {
				p.idx++
			}

			cmd := p.content[start:p.idx]
			var err error
			switch cmd {
			case "\\new-provider":
				err = p.buildNewProviderCommand()
			case "\\new-root":
				err = p.buildNewRootCommand()
			case "\\new-chat":
				err = p.buildNewChatCommand()
			case "\\chat":
				err = p.buildChatCommand()
			default:
				return fmt.Errorf("unknown command: %s", cmd)
			}

			if err != nil {
				return err
			}

			p.skipWhitespace()

			if p.idx < len(p.content) && p.content[p.idx] == '"' {
				if prop := p.parseString(); prop == nil {
					return fmt.Errorf("invalid command name string at position %d", p.idx)
				}
				p.skipWhitespace()
			}

			var requiredProps map[string]propertyType
			switch cmd {
			case "\\new-provider":
				requiredProps = map[string]propertyType{
					"host":          PropertyTypeString,
					"base-url":      PropertyTypeString,
					"max-tokens":    PropertyTypeInteger,
					"temperature":   PropertyTypeReal,
					"system-prompt": PropertyTypeString,
				}
			case "\\new-root":
				requiredProps = map[string]propertyType{
					"provider": PropertyTypeString,
				}
			case "\\new-chat":
				requiredProps = map[string]propertyType{
					"root": PropertyTypeString,
				}
			case "\\chat":
				requiredProps = map[string]propertyType{
					"restore": PropertyTypeString,
				}
			}
			return p.parseProperties(requiredProps)
		case ':':
			return nil
		default:
			p.idx++
		}
	}
	return nil
}

func (p *stmt) buildNewProviderCommand() error {
	p.cmd = &cmd{
		keyword:    "new-provider",
		properties: make(map[string]*property),
	}

	p.tokens = append(p.tokens, token{
		pos:       p.idx,
		tokenType: TokenTypeNewProviderCmd,
		value:     "\\new-provider",
	})

	// Required properties for new-provider
	requiredProps := map[string]propertyType{
		"host":          PropertyTypeString,
		"base-url":      PropertyTypeString,
		"max-tokens":    PropertyTypeInteger,
		"temperature":   PropertyTypeReal,
		"system-prompt": PropertyTypeString,
	}

	return p.parseProperties(requiredProps)
}

func (p *stmt) buildNewRootCommand() error {
	p.cmd = &cmd{
		keyword:    "new-root",
		properties: make(map[string]*property),
	}

	p.tokens = append(p.tokens, token{
		pos:       p.idx,
		tokenType: TokenTypeNewRootCmd,
		value:     "\\new-root",
	})

	// Required properties for new-root
	requiredProps := map[string]propertyType{
		"provider": PropertyTypeString,
	}

	return p.parseProperties(requiredProps)
}

func (p *stmt) buildNewChatCommand() error {
	p.cmd = &cmd{
		keyword:    "new-chat",
		properties: make(map[string]*property),
	}

	p.tokens = append(p.tokens, token{
		pos:       p.idx,
		tokenType: TokenTypeNewChatCmd,
		value:     "\\new-chat",
	})

	// Required properties for new-chat
	requiredProps := map[string]propertyType{
		"root": PropertyTypeString,
	}

	return p.parseProperties(requiredProps)
}

func (p *stmt) buildChatCommand() error {
	p.cmd = &cmd{
		keyword:    "chat",
		properties: make(map[string]*property),
	}

	p.tokens = append(p.tokens, token{
		pos:       p.idx,
		tokenType: TokenTypeChatCmd,
		value:     "\\chat",
	})

	// Required properties for chat
	requiredProps := map[string]propertyType{
		"restore": PropertyTypeString,
	}

	return p.parseProperties(requiredProps)
}

func (p *stmt) parseProperties(required map[string]propertyType) error {
	for p.idx < len(p.content) {
		p.skipWhitespace()

		if p.idx >= len(p.content) {
			break
		}

		if p.content[p.idx] != ':' {
			p.idx++
			continue
		}

		prop := p.parseProperty(required)
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

func (p *stmt) parseProperty(permitted map[string]propertyType) *property {
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

	typ, exists := permitted[propertyName]
	if !exists {
		return nil
	}

	var prop *property
	switch typ {
	case PropertyTypeString:
		// Special case for flag-like properties (e.g. :restore)
		if propertyName == "restore" {
			prop = &property{
				prop: propertyName,
				typ:  PropertyTypeString,
			}
		} else {
			prop = p.parseString()
		}
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

func (p *stmt) parseString() *property {
	if p.idx >= len(p.content) || p.content[p.idx] != '"' {
		return nil
	}

	start := p.idx
	p.idx++ // Skip opening quote

	for p.idx < len(p.content) {
		if p.content[p.idx] == '"' && (p.idx == 0 || p.content[p.idx-1] != '\\') {
			p.idx++ // Skip closing quote
			return &property{
				prop: p.content[start:p.idx],
				typ:  PropertyTypeString,
			}
		}
		p.idx++
	}

	return nil // Unterminated string
}

func (p *stmt) parseInteger() *property {
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

func (p *stmt) parseReal() *property {
	if p.idx >= len(p.content) {
		return nil
	}

	p.skipWhitespace()
	start := p.idx

	// Handle negative numbers
	if p.idx < len(p.content) && p.content[p.idx] == '-' {
		p.idx++
	}

	// Must have at least one digit before or after decimal
	hasDigits := false

	// Parse integer part
	for p.idx < len(p.content) && isDigit(p.content[p.idx]) {
		hasDigits = true
		p.idx++
	}

	// Parse decimal part if present
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
