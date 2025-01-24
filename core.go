package brunch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

/*
	Install the core to the directory that you want files and stuff to be placed and generated to
*/

const (
	dataStoreDirectory     = "data-store"
	chatStoreDirectory     = "chat-store"
	providerStoreDirectory = "provider-store"
)

// The brunch core handles the installes of and managment of chats and their related
// llm provider info. The core is what executes the statements and is used to load/store
// branchable chats
type Core struct {
	installDirectory string
	providers        map[string]Provider
	provMu           sync.Mutex

	sessions map[string]*coreSession
	sesMu    sync.Mutex

	activeChats map[string]*ChatInstance
	chatMu      sync.Mutex

	baseProviders map[string]Provider
}

type CoreOpts struct {
	InstallDirectory string
	BaseProviders    map[string]Provider
}

// The core handles the execution, and management-of chats and their related
// providers, data, etc. However, the core is NOT the chat instance itself.
// If a statement requests and audiance with an llm, the core chat request
// will be handed back in the CoreStmtExecResult following the ExecuteStatement
// call that requested it.
type CoreChatRequest struct {
	LoadedInstance *ChatInstance
}

type CoreStmtExecResult struct {
	Error       error
	ChatRequest *CoreChatRequest // This will be set iff \chat was called
}

// Create a new core instance with a set of
// providers that can be selected from. We are attempting to be
// entirely removed from the actual "chat" that the external
// user is doing, and instead we are just providing a way to
// manage instances of them, and add composability to the system
// through branching and traversal of a session forest
func NewCore(opts CoreOpts) *Core {
	return &Core{
		installDirectory: opts.InstallDirectory,
		providers:        opts.BaseProviders,
		sessions:         make(map[string]*coreSession),
		activeChats:      make(map[string]*ChatInstance),
		baseProviders:    opts.BaseProviders,
	}
}

func (c *Core) GetActiveChat(name string) (*ChatInstance, error) {
	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	chat, ok := c.activeChats[name]
	if !ok {
		return nil, fmt.Errorf("chat %s not found", name)
	}
	return chat, nil
}

func (c *Core) SetAvailableProviders(providers map[string]Provider) {
	c.providers = providers
}

// Sets up the core into the given install directory. It can be called multiple times
// and it wont overwrite the existing data store or chat store. It just makes sure that
// the directories exist that we rely on
func (c *Core) Install() error {
	if c.installDirectory == "" {
		return errors.New("install directory is required")
	}

	if c.IsInstalled() {
		return fmt.Errorf("target dir already exists: %s", c.installDirectory)
	}

	dirs := []string{
		filepath.Join(c.installDirectory, dataStoreDirectory),
		filepath.Join(c.installDirectory, chatStoreDirectory),
		filepath.Join(c.installDirectory, providerStoreDirectory),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) IsInstalled() bool {
	if c.installDirectory == "" {
		return false
	}
	_, err := os.Stat(c.installDirectory)
	return err == nil
}

func (c *Core) loadFromStore(store string, filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join(c.installDirectory, store, filename))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Retrive a list of all sessions
// This locks the session map and returns a list of all session ids, so
// best not call this in a hot spot
func (c *Core) SessionList() []string {
	c.sesMu.Lock()
	defer c.sesMu.Unlock()
	sessions := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		sessions = append(sessions, id)
	}
	return sessions
}

func (c *Core) EndSession(sessionId string) error {
	c.sesMu.Lock()
	defer c.sesMu.Unlock()
	_, ok := c.sessions[sessionId]
	if !ok {
		return fmt.Errorf("session %s not found", sessionId)
	}
	delete(c.sessions, sessionId)
	return nil
}

func (c *Core) ExecuteStatement(sessionId string, stmt *Statement) CoreStmtExecResult {

	if stmt == nil {
		return CoreStmtExecResult{Error: errors.New("statement is required")}
	}

	sanitized := strings.TrimSpace(sessionId)
	if sanitized == "" {
		return CoreStmtExecResult{Error: errors.New("session id is required")}
	}
	sessionId = sanitized

	var session *coreSession

	{
		var ok bool
		c.sesMu.Lock()
		session, ok = c.sessions[sessionId]
		if !ok {
			session = &coreSession{
				id: sessionId,
			}
			c.sessions[sessionId] = session
		}
		c.sesMu.Unlock()
	}

	var cr *CoreChatRequest
	callbacks := OperationalCallback{
		OnNewChat:     c.NewChat,
		OnNewProvider: c.newProviderFromStatement,
		OnLoadChat: func(name string, hash *string) error {
			ci, err := c.loadChat(name, hash)
			if err != nil {
				return err
			}
			cr = &CoreChatRequest{
				LoadedInstance: ci,
			}

			// Set the chat name in the session so we can track it from the user (who knows how many chats theyll have so we tie per-session)
			session.activeChatId = name
			return nil
		},
	}

	err := session.execute(stmt, callbacks)
	if err != nil {
		return CoreStmtExecResult{Error: err}
	}
	return CoreStmtExecResult{ChatRequest: cr}
}

// When the statement execution is done, the user may have executed a statement to create a new provider
// If this happens, we ensure that they are basing it off an existing (supported) provider, and then clone
// the settings to store in provider map
func (c *Core) newProviderFromStatement(name string, host string, baseUrl string, maxTokens int, temperature float64, systemPrompt string) error {

	fmt.Println("name:", name, "host", host)
	var baseProvider Provider
	{
		var exists bool
		c.provMu.Lock()
		_, exists = c.providers[name]
		if exists {
			c.provMu.Unlock()
			return fmt.Errorf("provider [%s] already exists", name)
		}

		baseProvider, exists = c.providers[host]
		if !exists {
			c.provMu.Unlock()
			return fmt.Errorf("host provider (base provider) [%s] does not exist", host)
		}
		c.provMu.Unlock()
	}

	if maxTokens == 0 || maxTokens > baseProvider.Settings().MaxTokens {
		fmt.Println("maxTokens is 0, setting to default")
		maxTokens = baseProvider.Settings().MaxTokens
	}

	if temperature == 0.0 || temperature > 1.0 {
		fmt.Println("temperature is 0 or greater than 1, setting to default")
		temperature = baseProvider.Settings().Temperature
	}

	// We "duplicate" checks, but who the fuck cares. Do this and save it to disk.
	return c.AddProvider(name, baseProvider.CloneWithSettings(ProviderSettings{
		Host:         host,
		BaseUrl:      baseUrl,
		MaxTokens:    maxTokens,
		Temperature:  temperature,
		SystemPrompt: systemPrompt,
	}))
}

// Here we clone the provider handed to us and store in the provider map under a new name
// given to us by the user so they can reference that particular incarnation of the provider
// in their chat sessions (host: is the base provider like "anthropic" or "openai" etc whatever is setup
// by hand from config oin core init)
func (c *Core) AddProvider(name string, p Provider) error {
	fmt.Println("Adding provider", name)

	// WHY DO YOU IGNORE LEXICAL SCOPES GOLANG?!?!?
	c.provMu.Lock()
	_, existsAlready := c.providers[name]
	if existsAlready {
		c.provMu.Unlock()
		return fmt.Errorf("provider [%s] already exists", name)
	}
	c.providers[name] = p
	c.provMu.Unlock()

	// Convert the settings to JSON format for saving to disk
	var settingsBytes []byte
	settings := p.Settings()
	var err error
	settingsBytes, err = json.Marshal(&settings)
	if err != nil {
		return fmt.Errorf("failed to marshal provider settings: %w", err)
	}

	// Save with a good, roman name, and then return
	sanitizedName := strings.ReplaceAll(name, " ", "_")
	return c.addToProviderStore(fmt.Sprintf("%s.json", sanitizedName), string(settingsBytes))
}

func (c *Core) LoadProviders() error {
	dataStoreDir := filepath.Join(c.installDirectory, providerStoreDirectory)
	files, err := os.ReadDir(dataStoreDir)
	if err != nil {
		return fmt.Errorf("failed to read provider store directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		fmt.Println("attempting to load ", file.Name())
		content, err := c.loadFromStore(providerStoreDirectory, file.Name())
		if err != nil {
			fmt.Println("failed to load provider file", file.Name())
			return fmt.Errorf("failed to load provider file %s: %w", file.Name(), err)
		}
		fmt.Println("loaded provider file", file.Name())

		var settings ProviderSettings
		if err := json.Unmarshal([]byte(content), &settings); err != nil {
			return fmt.Errorf("failed to unmarshal provider settings from %s: %w", file.Name(), err)
		}
		if _, exists := c.providers[settings.Name]; exists {
			return fmt.Errorf("provider %s already exists", settings.Name)
		}
		c.providers[settings.Name] = c.baseProviders["anthropic"].CloneWithSettings(settings)
	}
	return nil
}

// This creates a chat instance, but it does not load it. It defines it so that the user can
// load it later (think of it like making a db table)
func (c *Core) NewChat(name string, providerName string) error {
	var chat *ChatInstance
	{
		c.provMu.Lock()
		defer c.provMu.Unlock()

		provider, ok := c.providers[providerName]

		if !ok {
			for name, prov := range c.providers {
				fmt.Println("PROVIDER", name, prov.Settings().Name)
			}
			return fmt.Errorf("provider [%s] not found", providerName)
		}

		baseSettings := provider.Settings()
		baseSettings.Name = name

		chat = NewChatInstance(provider.CloneWithSettings(baseSettings))
	}

	return c.writeSnapshot(name, chat)
}

func (c *Core) SaveActiveChat(sessionName string) error {
	var target string
	var chat *ChatInstance
	{
		c.sesMu.Lock()
		session, exists := c.sessions[sessionName]
		c.sesMu.Unlock()

		if !exists {
			return fmt.Errorf("session [%s] does not exist", sessionName)
		}

		target = session.activeChatId

		c.chatMu.Lock()
		chat, exists = c.activeChats[target]
		c.chatMu.Unlock()

		if !exists {
			return fmt.Errorf("chat [%s] is not active", target)
		}
	}
	return c.writeSnapshot(target, chat)
}

func (c *Core) writeSnapshot(ssName string, chat *ChatInstance) error {
	ss, err := chat.Snapshot()
	if err != nil {
		return err
	}
	data, err := ss.Marshal()
	if err != nil {
		return err
	}
	if err := c.AddToChatStore(fmt.Sprintf("%s.json", ssName), string(data)); err != nil {
		return err
	}
	return nil
}

func (c *Core) loadChat(name string, hash *string) (*ChatInstance, error) {
	{
		c.chatMu.Lock()
		chat, exists := c.activeChats[name]
		c.chatMu.Unlock()
		if exists {
			return chat, nil
		}
	}

	fileName := name
	if !strings.HasSuffix(fileName, ".json") {
		fileName = fmt.Sprintf("%s.json", name)
	}

	snapshotRaw, err := c.LoadFromChatStore(fileName)
	if err != nil {
		return nil, err
	}
	var snapshot Snapshot
	err = json.Unmarshal([]byte(snapshotRaw), &snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat snapshot: %w", err)
	}
	chat, err := NewChatInstanceFromSnapshot(c.providers, &snapshot)
	if err != nil {
		return nil, err
	}

	// Restore to last point in chat
	if hash != nil {
		chat.Goto(*hash)
	}

	// Add to active chats
	{
		c.chatMu.Lock()
		c.activeChats[name] = chat
		c.chatMu.Unlock()
	}
	return chat, nil
}

func (c *Core) AddToDataStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, dataStoreDirectory, filename), content)
}

func (c *Core) AddToChatStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, chatStoreDirectory, filename), content)
}

func (c *Core) addData(filename string, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func (c *Core) addToProviderStore(filename string, content string) error {
	return os.WriteFile(filepath.Join(c.installDirectory, providerStoreDirectory, filename), []byte(content), 0644)
}

func (c *Core) LoadFromDataStore(filename string) (string, error) {
	return c.loadFromStore(dataStoreDirectory, filename)
}

func (c *Core) LoadFromChatStore(filename string) (string, error) {
	return c.loadFromStore(chatStoreDirectory, filename)
}
