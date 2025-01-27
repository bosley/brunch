package brunch

import (
	"errors"
	"fmt"
	"strconv"
)

// An operational callback is used when a session with a user (pre-chat interface) is in process.
// When they submit a commmand via the core, it will use these callbacks to receive instructions
// based on the command when `execucte` is called (below)
type OperationalCallback struct {
	OnLoadChat        func(name string, hash *string) error
	OnNewChat         func(name string, provider string) error
	OnNewProvider     func(name string, host string, baseUrl string, maxTokens int, temperature float64, systemPrompt string) error
	OnNewContext      func(name string, dir *string, database *string, web *string) error
	OnDeleteChat      func(name string) error
	OnDeleteContext   func(name string) error
	OnListChats       func() error
	OnListContexts    func() error
	OnDescribeContext func(name string) error
	OnDescribeChat    func(name string) error
}

type coreSession struct {
	id           string
	activeChatId string
}

// Send a statement to the session (called by the core)
func (s *coreSession) execute(stmt *Statement, callbacks OperationalCallback) error {

	if !stmt.IsPrepared() {
		if err := stmt.Prepare(); err != nil {
			return err
		}
	}

	if err := s.validateProperties(stmt); err != nil {
		return err
	}

	// map for restriction validation on per-command basis
	propertyMap := make(map[string]*property)
	for _, prop := range stmt.cmd.properties {
		propertyMap[prop.id] = prop
	}

	switch stmt.cmd.keyword {
	case "new-provider":
		return s.newProvider(stmt.cmd.nameGiven, propertyMap, callbacks)
	case "new-chat":
		return s.newChat(stmt.cmd.nameGiven, propertyMap, callbacks)
	case "chat":
		return s.chat(stmt.cmd.nameGiven, propertyMap, callbacks)
	case "new-ctx":
		return s.newContext(stmt.cmd.nameGiven, propertyMap, callbacks)
	case "del-chat":
		return s.deleteChat(stmt.cmd.nameGiven, callbacks)
	case "del-ctx":
		return s.deleteContext(stmt.cmd.nameGiven, callbacks)
	case "list-chat":
		return s.listChats(callbacks)
	case "list-ctx":
		return s.listContexts(callbacks)
	case "desc-ctx":
		return s.describeContext(stmt.cmd.nameGiven, callbacks)
	case "desc-chat":
		return s.describeChat(stmt.cmd.nameGiven, callbacks)
	}

	return errors.New("not implemented")
}

func (s *coreSession) validateProperties(stmt *Statement) error {
	for _, prop := range stmt.cmd.properties {
		if !s.isPropertyValid(prop) {
			return fmt.Errorf("invalid property: %s", prop.id)
		}
	}
	return nil
}

func (s *coreSession) isPropertyValid(p *property) bool {
	switch p.typ {
	case PropertyTypeString:
		return p.prop != ""
	case PropertyTypeInteger:
		_, err := strconv.Atoi(p.prop)
		return err == nil
	case PropertyTypeReal:
		_, err := strconv.ParseFloat(p.prop, 64)
		return err == nil
	}
	return false
}

func (s *coreSession) newProvider(name string, propertyMap map[string]*property, callbacks OperationalCallback) error {

	var err error
	var host string
	var baseUrl string
	var maxTokens int
	var temperature float64
	var systemPrompt string

	for key, prop := range propertyMap {
		switch key {
		case "host":
			if prop.typ != PropertyTypeString {
				return fmt.Errorf("host must be a string")
			}
			host = prop.prop
		case "base-url":
			if prop.typ != PropertyTypeString {
				return fmt.Errorf("base-url must be a string")
			}
			baseUrl = prop.prop
		case "max-tokens":
			if prop.typ != PropertyTypeInteger {
				return fmt.Errorf("max-tokens must be an integer")
			}
			maxTokens, err = strconv.Atoi(prop.prop)
			if err != nil {
				return fmt.Errorf("max-tokens must be an integer")
			}
		case "temperature":
			if prop.typ != PropertyTypeReal {
				return fmt.Errorf("temperature must be a real number")
			}
			temperature, err = strconv.ParseFloat(prop.prop, 64)
			if err != nil {
				return fmt.Errorf("temperature must be a real number")
			}
		case "system-prompt":
			if prop.typ != PropertyTypeString {
				return fmt.Errorf("system-prompt must be a string")
			}
			systemPrompt = prop.prop
		default:
			return fmt.Errorf("invalid, unknown property: %s", key)
		}
	}

	if name == "" {
		return fmt.Errorf("name must be specified")
	}

	// We have to call into the core to create the provider it is the one that hosts
	// the controlled map of providers that can be selected from as we have a hard
	// seperation between provider implementations and the core
	// the core will validate the properties data
	return callbacks.OnNewProvider(name, host, baseUrl, maxTokens, temperature, systemPrompt)
}

func (s *coreSession) newChat(name string, propertyMap map[string]*property, callbacks OperationalCallback) error {

	var provider string

	for key, prop := range propertyMap {
		switch key {
		case "provider":
			provider = prop.prop
		default:
			return fmt.Errorf("invalid, unknown property: %s", key)
		}
	}

	if provider == "" {
		return fmt.Errorf("provider must be specified")
	}

	if name == "" {
		return fmt.Errorf("name must be specified")
	}

	return callbacks.OnNewChat(name, provider)
}

func (s *coreSession) chat(name string, propertyMap map[string]*property, callbacks OperationalCallback) error {

	var hash *string

	for key, prop := range propertyMap {
		switch key {
		case "hash":
			hash = &prop.prop
		default:
			return fmt.Errorf("invalid, unknown property: %s", key)
		}
	}

	if name == "" {
		return fmt.Errorf("name must be specified")
	}

	return callbacks.OnLoadChat(name, hash)
}

func (s *coreSession) newContext(name string, propertyMap map[string]*property, callbacks OperationalCallback) error {

	var dir *string
	var database *string
	var web *string

	for key, prop := range propertyMap {
		switch key {
		case "dir":
			dir = &prop.prop
		case "database":
			database = &prop.prop
		case "web":
			web = &prop.prop
		default:
			return fmt.Errorf("invalid, unknown property: %s", key)
		}
	}

	if name == "" {
		return fmt.Errorf("name must be specified")
	}

	return callbacks.OnNewContext(name, dir, database, web)
}

func (s *coreSession) deleteChat(name string, callbacks OperationalCallback) error {
	if name == "" {
		return fmt.Errorf("name must be specified")
	}
	return callbacks.OnDeleteChat(name)
}

func (s *coreSession) deleteContext(name string, callbacks OperationalCallback) error {
	if name == "" {
		return fmt.Errorf("name must be specified")
	}
	return callbacks.OnDeleteContext(name)
}

func (s *coreSession) listChats(callbacks OperationalCallback) error {
	return callbacks.OnListChats()
}

func (s *coreSession) listContexts(callbacks OperationalCallback) error {
	return callbacks.OnListContexts()
}

func (s *coreSession) describeContext(name string, callbacks OperationalCallback) error {
	if name == "" {
		return fmt.Errorf("name must be specified")
	}
	return callbacks.OnDescribeContext(name)
}

func (s *coreSession) describeChat(name string, callbacks OperationalCallback) error {
	if name == "" {
		return fmt.Errorf("name must be specified")
	}
	return callbacks.OnDescribeChat(name)
}
