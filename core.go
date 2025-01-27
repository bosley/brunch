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
	contextStoreDirectory  = "context-store"
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

	activeChats map[string]*chatInstance
	chatMu      sync.Mutex

	baseProviders map[string]Provider

	contexts map[string]*ContextSettings
	ctxMu    sync.Mutex
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
	LoadedInstance *chatInstance
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
		activeChats:      make(map[string]*chatInstance),
		baseProviders:    opts.BaseProviders,
		contexts:         make(map[string]*ContextSettings),
	}
}

func (c *Core) GetActiveChat(name string) (*chatInstance, error) {
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
		filepath.Join(c.installDirectory, contextStoreDirectory),
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
		OnNewChat:         c.NewChat,
		OnNewProvider:     c.newProviderFromStatement,
		OnNewContext:      c.newContext,
		OnListChats:       c.onListChats,
		OnListContexts:    c.onListContexts,
		OnDescribeContext: c.onDescribeContext,
		OnDescribeChat:    c.onDescribeChat,
		OnListProviders:   c.onListProviders,
		OnDeleteProvider:  c.onDeleteProvider,
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
		OnDeleteChat:    c.deleteChat,
		OnDeleteContext: c.deleteContext,
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
		Name:         name,
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

// Load all available providers from the provider store directory
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

func (c *Core) LoadContexts() error {
	dataStoreDir := filepath.Join(c.installDirectory, contextStoreDirectory)
	files, err := os.ReadDir(dataStoreDir)
	if err != nil {
		return fmt.Errorf("failed to read context store directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		content, err := c.loadFromStore(contextStoreDirectory, file.Name())
		if err != nil {
			return fmt.Errorf("failed to load context file %s: %w", file.Name(), err)
		}

		var ctx ContextSettings
		if err := json.Unmarshal([]byte(content), &ctx); err != nil {
			return fmt.Errorf("failed to unmarshal context settings from %s: %w", file.Name(), err)
		}

		c.contexts[ctx.Name] = &ctx
	}
	return nil
}

// This creates a chat instance, but it does not load it. It defines it so that the user can
// load it later (think of it like making a db table)
func (c *Core) NewChat(name string, providerName string) error {
	var chat *chatInstance
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

		chatSettings := provider.Settings()
		chatSettings.Name = name
		chatSettings.Host = providerName
		cloned := provider.CloneWithSettings(chatSettings)
		chat = newChatInstance(cloned)
	}

	return c.writeSnapshot(name, chat)
}

func (c *Core) SaveActiveChat(sessionName string) error {
	var target string
	var chat *chatInstance
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

func (c *Core) writeSnapshot(ssName string, chat *chatInstance) error {
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

func (c *Core) loadChat(name string, hash *string) (*chatInstance, error) {
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
	chat, err := newChatInstanceFromSnapshot(c, &snapshot)
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

func (c *Core) newContext(name string, dir *string, database *string, web *string) error {
	ctx := ContextSettings{
		Name: name,
	}
	if dir != nil {
		ctx.Type = ContextTypeDirectory
		ctx.Value = *dir
	} else if database != nil {
		ctx.Type = ContextTypeDatabase
		ctx.Value = *database
	} else if web != nil {
		ctx.Type = ContextTypeWeb
		ctx.Value = *web
	}
	content, err := json.Marshal(ctx)
	if err != nil {
		return err
	}

	c.ctxMu.Lock()
	if _, exists := c.contexts[name]; exists {
		c.ctxMu.Unlock()
		return fmt.Errorf("context %s already exists", name)
	}

	if err := c.AddToContextStore(fmt.Sprintf("%s.json", name), string(content)); err != nil {
		c.ctxMu.Unlock()
		return err
	}

	c.contexts[name] = &ctx
	c.ctxMu.Unlock()
	return nil
}

func (c *Core) newContextFromAttached(ctx *ContextSettings) error {
	content, err := json.Marshal(ctx)
	if err != nil {
		return err
	}
	return c.AddToContextStore(fmt.Sprintf("%s.json", ctx.Name), string(content))
}

func (c *Core) addData(filename string, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func (c *Core) AddToDataStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, dataStoreDirectory, filename), content)
}

func (c *Core) AddToChatStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, chatStoreDirectory, filename), content)
}

func (c *Core) addToProviderStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, providerStoreDirectory, filename), content)
}

func (c *Core) loadFromStore(store string, filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join(c.installDirectory, store, filename))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (c *Core) LoadFromDataStore(filename string) (string, error) {
	return c.loadFromStore(dataStoreDirectory, filename)
}

func (c *Core) LoadFromChatStore(filename string) (string, error) {
	return c.loadFromStore(chatStoreDirectory, filename)
}

func (c *Core) LoadFromContextStore(filename string) (string, error) {
	return c.loadFromStore(contextStoreDirectory, filename)
}

func (c *Core) AddToContextStore(filename string, content string) error {
	return c.addData(filepath.Join(c.installDirectory, contextStoreDirectory, filename), content)
}

// isContextInUse checks if a context is being used by any chat by scanning all chat files
func (c *Core) isContextInUse(contextName string) (bool, error) {
	chatStoreDir := filepath.Join(c.installDirectory, chatStoreDirectory)
	files, err := os.ReadDir(chatStoreDir)
	if err != nil {
		return false, fmt.Errorf("failed to read chat store directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		content, err := c.LoadFromChatStore(file.Name())
		if err != nil {
			return false, fmt.Errorf("failed to load chat file %s: %w", file.Name(), err)
		}

		var snapshot Snapshot
		if err := json.Unmarshal([]byte(content), &snapshot); err != nil {
			return false, fmt.Errorf("failed to unmarshal chat snapshot from %s: %w", file.Name(), err)
		}

		// Check if this chat uses the context
		for _, ctx := range snapshot.Contexts {
			if ctx == contextName {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *Core) deleteChat(name string) error {
	// First check if the chat is active in any session
	c.sesMu.Lock()
	for _, session := range c.sessions {
		if session.activeChatId == name {
			c.sesMu.Unlock()
			return fmt.Errorf("cannot delete chat %s: it is currently active in a session", name)
		}
	}
	c.sesMu.Unlock()

	// Check if it's in active chats
	c.chatMu.Lock()
	if _, exists := c.activeChats[name]; exists {
		c.chatMu.Unlock()
		return fmt.Errorf("cannot delete chat %s: it is currently active", name)
	}
	c.chatMu.Unlock()

	// Delete the chat file
	chatFile := fmt.Sprintf("%s.json", name)
	if !strings.HasSuffix(name, ".json") {
		chatFile = fmt.Sprintf("%s.json", name)
	}

	err := os.Remove(filepath.Join(c.installDirectory, chatStoreDirectory, chatFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete chat file: %w", err)
	}

	return nil
}

func (c *Core) deleteContext(name string) error {
	// First check if the context exists
	c.ctxMu.Lock()
	_, exists := c.contexts[name]
	if !exists {
		c.ctxMu.Unlock()
		return fmt.Errorf("context %s does not exist", name)
	}
	c.ctxMu.Unlock()

	// Check if the context is in use by any chat
	inUse, err := c.isContextInUse(name)
	if err != nil {
		return fmt.Errorf("failed to check if context is in use: %w", err)
	}
	if inUse {
		return fmt.Errorf("cannot delete context %s: it is currently in use by one or more chats", name)
	}

	// Remove from memory
	c.ctxMu.Lock()
	delete(c.contexts, name)
	c.ctxMu.Unlock()

	// Delete the context file
	contextFile := fmt.Sprintf("%s.json", name)
	if !strings.HasSuffix(name, ".json") {
		contextFile = fmt.Sprintf("%s.json", name)
	}

	err = os.Remove(filepath.Join(c.installDirectory, contextStoreDirectory, contextFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete context file: %w", err)
	}

	return nil
}

func (c *Core) getStorageJsons(store string) ([]string, error) {
	storeDir := filepath.Join(c.installDirectory, store)
	files, err := os.ReadDir(storeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s store directory: %w", store, err)
	}

	jsons := []string{}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		jsons = append(jsons, file.Name())
	}

	return jsons, nil
}

func (c *Core) onDeleteProvider(name string) error {
	// First check if the provider exists
	c.provMu.Lock()
	_, exists := c.providers[name]
	if !exists {
		c.provMu.Unlock()
		return fmt.Errorf("provider %s does not exist", name)
	}

	// Check if it's a base provider
	if _, isBase := c.baseProviders[name]; isBase {
		c.provMu.Unlock()
		return fmt.Errorf("cannot delete base provider %s", name)
	}

	// Check if any chats are using this provider
	inUse := false
	chatStoreDir := filepath.Join(c.installDirectory, chatStoreDirectory)
	files, err := os.ReadDir(chatStoreDir)
	if err != nil {
		c.provMu.Unlock()
		return fmt.Errorf("failed to read chat store directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		content, err := c.LoadFromChatStore(file.Name())
		if err != nil {
			c.provMu.Unlock()
			return fmt.Errorf("failed to load chat file %s: %w", file.Name(), err)
		}

		var snapshot Snapshot
		if err := json.Unmarshal([]byte(content), &snapshot); err != nil {
			c.provMu.Unlock()
			return fmt.Errorf("failed to unmarshal chat snapshot from %s: %w", file.Name(), err)
		}

		if snapshot.ProviderName == name {
			inUse = true
			break
		}
	}

	if inUse {
		c.provMu.Unlock()
		return fmt.Errorf("cannot delete provider %s: it is currently in use by one or more chats", name)
	}

	// Remove from memory
	delete(c.providers, name)
	c.provMu.Unlock()

	// Delete the provider file
	providerFile := fmt.Sprintf("%s.json", name)
	if !strings.HasSuffix(name, ".json") {
		providerFile = fmt.Sprintf("%s.json", name)
	}

	err = os.Remove(filepath.Join(c.installDirectory, providerStoreDirectory, providerFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete provider file: %w", err)
	}

	return nil
}

func (c *Core) ListContexts() []string {
	c.ctxMu.Lock()
	defer c.ctxMu.Unlock()
	ctxs := make([]string, 0, len(c.contexts))
	for name := range c.contexts {
		ctxs = append(ctxs, name)
	}
	return ctxs
}

func (c *Core) onListChats() ([]string, error) {
	jsons, err := c.getStorageJsons(chatStoreDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat store jsons: %w", err)
	}

	chats := []string{}
	for _, json := range jsons {
		name := strings.TrimSuffix(json, ".json")
		chats = append(chats, name)
	}
	return chats, nil
}

func (c *Core) onListContexts() ([]string, error) {
	jsons, err := c.getStorageJsons(contextStoreDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to get context store jsons: %w", err)
	}

	ctxs := []string{}
	for _, json := range jsons {
		name := strings.TrimSuffix(json, ".json")
		ctxs = append(ctxs, name)
	}
	return ctxs, nil
}

func (c *Core) onDescribeContext(name string) (string, error) {

	if !strings.HasSuffix(name, ".json") {
		name = fmt.Sprintf("%s.json", name)
	}

	content, err := c.LoadFromContextStore(name)
	if err != nil {
		return "", fmt.Errorf("failed to load context from disk: %w", err)
	}
	return content, nil
}

func (c *Core) onDescribeChat(name string) (string, error) {
	chat, err := c.loadChat(name, nil)
	if err != nil {
		return "", fmt.Errorf("failed to load chat from disk: %w", err)
	}

	desc := fmt.Sprintf("%-15s %s\n", "Name:", name)
	desc += fmt.Sprintf("%-15s %s\n", "Provider:", chat.provider.Settings().Name)
	desc += fmt.Sprintf("%-15s %s\n", "Base URL:", chat.provider.Settings().BaseUrl)
	desc += fmt.Sprintf("%-15s %d\n", "Max Tokens:", chat.provider.Settings().MaxTokens)
	desc += fmt.Sprintf("%-15s %.2f\n", "Temperature:", chat.provider.Settings().Temperature)
	desc += fmt.Sprintf("%-15s %s\n", "System Prompt:", chat.provider.Settings().SystemPrompt)
	desc += fmt.Sprintf("%-15s %d\n", "Contexts:", len(chat.contexts))
	for _, ctx := range chat.contexts {
		desc += fmt.Sprintf("%-15s %s\n", "", ctx.Name)
	}
	desc += fmt.Sprintf("%-15s %s\n", "Active Hash:", chat.currentNode.Hash())
	return desc, nil
}

func (c *Core) onListProviders() ([]string, error) {
	c.provMu.Lock()
	defer c.provMu.Unlock()

	jsons, err := c.getStorageJsons(providerStoreDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider store jsons: %w", err)
	}

	providers := []string{}
	providers = append(providers, fmt.Sprintf("Base Providers (immutable): %d", len(c.baseProviders)))
	for _, prov := range c.baseProviders {
		providers = append(providers, fmt.Sprintf("\t%s", prov.Settings().Name))
	}

	providers = append(providers, "\n\nDerived Providers:")
	for _, json := range jsons {
		name := strings.TrimSuffix(json, ".json")
		providers = append(providers, fmt.Sprintf("\t%s", name))
	}

	return providers, nil
}
