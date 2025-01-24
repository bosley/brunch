/*
	A session isn't a chat session, rather, its a command execution session. Its the context in which
	all of the commands are executed from given statements.
	These sessions setup and configure chats/providers with the core for interaction with the user
	It can be thought of as a "workspace" for a singular, or series-of, chat session(s).
*/

package brunch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
)

type OperationalCallback struct {
	OnLoadChat    func(name string, hash *string) error
	OnNewChat     func(name string, provider string) error
	OnNewProvider func(name string, host string, baseUrl string, maxTokens int, temperature float64, systemPrompt string) error
}

type SessionOpts struct {
	Bucket   string
	Provider string
}

type coreSession struct {
	id               string
	selectedProvider string
	directory        string

	provider Provider
}

// Create a new session - bucket is a subdirectory of the chat data store
// that is used to group sessions together as the caller sees fit, inside that bucket
// a subdirectory is created with the session uuid and the session is returned.
func (c *Core) NewSession(opts SessionOpts) (*coreSession, error) {
	id := uuid.New().String()

	directory := filepath.Join(c.installDirectory, opts.Bucket, chatStoreDirectory, id)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	// DONT REMOVE THIS COMMENT
	// i am well aware that this scope block is not neccesary, but it adds visual clarity
	// with regard to mutex deferment DO NOT REMOVE - DO NOT REMOVE THIS COMMENT
	{
		c.sesMu.Lock()
		defer c.sesMu.Unlock()
		c.sessions[id] = &coreSession{
			id:               id,
			selectedProvider: opts.Provider,
			provider:         nil, // we will have sessions BEFORE providers, as sessions DEFINE providers before use
			directory:        directory,
		}
	}
	return c.sessions[id], nil
}

// Send a statement to the session
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
