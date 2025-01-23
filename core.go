package brunch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

/*
	Install the core to the directory that you want files and stuff to be placed and generated to
*/

const (
	dataStoreDirectory = "data-store"
	chatStoreDirectory = "chat-store"
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
}

type CoreOpts struct {
	InstallDirectory string
	Providers        map[string]Provider
}

// The core handles the execution, and management-of chats and their related
// providers, data, etc. However, the core is NOT the chat instance itself.
// If a statement requests and audiance with an llm, the core chat request
// will be handed back in the CoreStmtExecResult following the ExecuteStatement
// call that requested it.
type CoreChatRequest struct {
	ChatName string // TODO: Once we actually go to do this, since the core stores the chats, we will load the object the external caller can utilize
	ChatHash *string
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
		providers:        make(map[string]Provider),
		sessions:         make(map[string]*coreSession),
	}
}

// Sets up the core into the given install directory. It can be called multiple times
// and it wont overwrite the existing data store or chat store. It just makes sure that
// the directories exist that we rely on
func (c *Core) Install() error {
	if c.installDirectory == "" {
		return errors.New("install directory is required")
	}

	dirs := []string{
		filepath.Join(c.installDirectory, dataStoreDirectory),
		filepath.Join(c.installDirectory, chatStoreDirectory),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) AddToDataStore(filename string, content string) error {
	return c.addToDataStore(filepath.Join(c.installDirectory, dataStoreDirectory, filename), content)
}

func (c *Core) AddToChatStore(filename string, content string) error {
	return c.addToDataStore(filepath.Join(c.installDirectory, chatStoreDirectory, filename), content)
}

func (c *Core) addToDataStore(filename string, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func (c *Core) LoadFromDataStore(filename string) (string, error) {
	return c.loadFromStore(dataStoreDirectory, filename)
}

func (c *Core) LoadFromChatStore(filename string) (string, error) {
	return c.loadFromStore(chatStoreDirectory, filename)
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

// TODO: Submit the Statement (maybe make an explicit PreparedStatement) to the
// tree structure stuff and save the session - this is the manager for all active chat session
// on the system. They will all center around the same install directory but may use different
// providers to undergo taking on varyhing, distinct tasks under the project.
func (c *Core) ExecuteStatement(sessionId string, stmt Statement) CoreStmtExecResult {
	c.sesMu.Lock()
	defer c.sesMu.Unlock()
	session, ok := c.sessions[sessionId]
	if !ok {
		return CoreStmtExecResult{Error: fmt.Errorf("session %s not found", sessionId)}
	}

	var cr *CoreChatRequest
	callbacks := OperationalCallback{
		OnNewChat:     c.NewChat,
		OnNewProvider: c.AddProvider,
		OnLoadChat: func(name string, hash *string) error {
			cr = &CoreChatRequest{
				ChatName: name,
				ChatHash: hash,
			}
			return nil
		},
	}

	err := session.execute(stmt, callbacks)
	if err != nil {
		return CoreStmtExecResult{Error: err}
	}
	return CoreStmtExecResult{ChatRequest: cr}
}

// Here we clone the provider handed to us and store in the provider map under a new name
// given to us by the user so they can reference that particular incarnation of the provider
// in their chat sessions (host: is the base provider like "anthropic" or "openai" etc whatever is setup
// by hand from config oin core init)
func (c *Core) AddProvider(name string, host string, baseUrl string, maxTokens int, temperature float64, systemPrompt string) error {
	fmt.Println("Adding provider", name, host, baseUrl, maxTokens, temperature, systemPrompt)

	c.provMu.Lock()
	defer c.provMu.Unlock()

	_, existsAlready := c.providers[name]
	if existsAlready {
		return fmt.Errorf("provider [%s] already exists", name)
	}
	base, ok := c.providers[host]
	if !ok {
		return fmt.Errorf("host provider [%s] not found", host)
	}
	provider := base.CloneWithSettings(ProviderSettings{
		BaseUrl:      baseUrl,
		MaxTokens:    maxTokens,
		Temperature:  temperature,
		SystemPrompt: systemPrompt,
	})
	c.providers[name] = provider
	return nil
}

// This creates a chat instance, but it does not load it. It defines it so that the user can
// load it later (think of it like making a db table)
func (c *Core) NewChat(name string, providerName string) error {

	c.provMu.Lock()
	defer c.provMu.Unlock()

	provider, ok := c.providers[providerName]
	if !ok {
		return fmt.Errorf("provider [%s] not found", providerName)
	}

	fmt.Println("NewChat", name, provider)
	return errors.New("not implemented")
}
