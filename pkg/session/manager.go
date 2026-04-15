package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents an active vshell session
type Session struct {
	ID         string
	CreatedAt  time.Time
	LastActive time.Time
	Channels   map[uint8]*Channel
	Data       map[string]interface{} // Store shell, file transfer state, etc.
	Conn       interface{}            // *protocol.Conn reference
	mu         sync.RWMutex
}

// Channel represents a logical channel for multiplexed communication
type Channel struct {
	ID      uint8
	Type    string // "shell", "file"
	Handler interface{}
	SendBuf []byte
	RecvBuf []byte
	Open    bool
}

// Manager manages all active sessions
type Manager struct {
	sessions    map[string]*Session
	mu          sync.RWMutex
	maxSessions int
	timeout     time.Duration
}

// NewManager creates a new session manager
func NewManager(maxSessions int, timeout time.Duration) *Manager {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	m := &Manager{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
		timeout:     timeout,
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// Create creates a new session
func (m *Manager) Create() (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxSessions {
		return nil, fmt.Errorf("maximum sessions reached (%d)", m.maxSessions)
	}

	id := uuid.New().String()
	now := time.Now()

	sess := &Session{
		ID:         id,
		CreatedAt:  now,
		LastActive: now,
		Channels:   make(map[uint8]*Channel),
		Data:       make(map[string]interface{}),
	}

	m.sessions[id] = sess
	return sess, nil
}

// Get retrieves a session by ID
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return sess, nil
}

// UpdateActivity updates the last active timestamp
func (m *Manager) UpdateActivity(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	sess.LastActive = time.Now()
	return nil
}

// Destroy removes a session
func (m *Manager) Destroy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// Close channels
	for _, ch := range sess.Channels {
		ch.Open = false
	}

	delete(m.sessions, id)
	return nil
}

// List returns all active session IDs
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active sessions
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// AddChannel adds a channel to a session
func (s *Session) AddChannel(id uint8, chType string, handler interface{}) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Channels[id]; exists {
		return nil, fmt.Errorf("channel %d already exists", id)
	}

	ch := &Channel{
		ID:      id,
		Type:    chType,
		Handler: handler,
		Open:    true,
	}

	s.Channels[id] = ch
	return ch, nil
}

// GetChannel retrieves a channel by ID
func (s *Session) GetChannel(id uint8) (*Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.Channels[id]
	if !ok {
		return nil, fmt.Errorf("channel not found: %d", id)
	}

	return ch, nil
}

// RemoveChannel removes a channel
func (s *Session) RemoveChannel(id uint8) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.Channels[id]; ok {
		ch.Open = false
		delete(s.Channels, id)
	}

	return nil
}

// SetData stores session data
func (s *Session) SetData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
}

// GetData retrieves session data
func (s *Session) GetData(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Data[key]
	return val, ok
}

// cleanupLoop removes expired sessions
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup removes expired sessions
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, sess := range m.sessions {
		if now.Sub(sess.LastActive) > m.timeout {
			// Cleanup channels
			for _, ch := range sess.Channels {
				ch.Open = false
			}
			delete(m.sessions, id)
		}
	}
}
