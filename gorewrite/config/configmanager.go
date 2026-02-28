package config

import (
	"errors"
	"strings"
	"sync"

	"github.com/wxcuop/pyfixmsg_plus/crypt"
	"gopkg.in/ini.v1"
)

// Manager is a thread-safe singleton for reading INI config files.
type Manager struct {
	mu  sync.RWMutex
	cfg *ini.File
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager returns the singleton Config Manager.
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{}
	})
	return instance
}

// Load reads the INI file at path.
func (m *Manager) Load(path string) error {
	f, err := ini.Load(path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = f
	m.mu.Unlock()
	return nil
}

// MustLoad is like Load but panics on error. Useful for tests.
func (m *Manager) MustLoad(path string) {
	if err := m.Load(path); err != nil {
		panic(err)
	}
}

// Get returns the raw value for section.key. If section is empty, uses DEFAULT section.
func (m *Manager) Get(section, key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return ""
	}
	if section == "" {
		return m.cfg.Section("").Key(key).String()
	}
	return m.cfg.Section(section).Key(key).String()
}

// GetDecrypted returns the value for section.key, decrypting values with the "ENC:" prefix
// using the provided passphrase. If the value is not prefixed, it is returned as-is.
func (m *Manager) GetDecrypted(section, key, passphrase string) (string, error) {
	v := m.Get(section, key)
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, "ENC:") {
		b := strings.TrimPrefix(v, "ENC:")
		if passphrase == "" {
			return "", errors.New("config: encrypted value but empty passphrase")
		}
		return crypt.DecryptString(b, passphrase)
	}
	return v, nil
}
