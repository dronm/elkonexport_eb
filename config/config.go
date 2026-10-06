// Package config
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	defMaxBackups = 5
	defMaxMBSize  = 10
	defMaxDayAge  = 30
)

type Auth struct {
	URL string `json:"url"`
	// Name    string `json:"name"`
	// Passord string `json:"password"`
}

type UnclosedProductionList map[int]time.Time

type Production struct {
	QueryProductionCount      int                    `json:"query_production_count"`
	LastProduction            int                    `json:"last_production"`
	ProductionSiteID          int                    `json:"production_site_id"`
	ProductionSiteDescr       string                 `json:"production_site_descr"`
	CheckUnclosedProductionsH int                    `json:"check_unclosed_productions_h"`
	UnclosedProductions       UnclosedProductionList `json:"unclosed_productions"`
}

type LogRotation struct {
	MaxMBSize  int  `json:"max_mb_size"`
	MaxBackups int  `json:"max_backups"`
	MaxDayAge  int  `json:"max_day_age"`
	Compress   bool `json:"compress"`
}

type Config struct {
	WebSocketURL             string     `json:"websocket_url"`
	MSSQLConnStr             string     `json:"mssql_conn_str"`
	QueryIntervalS           int        `json:"query_interval_s"`
	EmptyDebounceS           int        `json:"empty_debounce_s"`
	ConnectRetryS            int        `json:"connect_retry_s"`
	ProductionErrorDebounceS int        `json:"production_error_debounce_s"`
	ShutdownS                int        `json:"shutdown_s"`
	LogLevel                 string     `json:"log_level"`
	LogFileName              string     `json:"log_file_name"`
	Auth                     Auth       `json:"auth"`
	Production               Production `json:"production"`
	LogRotation              LogRotation
}

// Manager handles config with safe concurrent access
type Manager struct {
	FileName string
	mu       sync.RWMutex
	cfg      Config
}

func NewManager(fileName string) *Manager {
	return &Manager{FileName: fileName}
}

// Load loads config from file.
func (m *Manager) Load() error {
	if _, err := os.Stat(m.FileName); err != nil {
		return err
	}
	// load existing
	b, err := os.ReadFile(m.FileName)
	if err != nil {
		return fmt.Errorf("read cfg: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return fmt.Errorf("unmarshal cfg: %w", err)
	}

	if c.LogRotation.MaxBackups == 0 {
		c.LogRotation.MaxBackups = defMaxBackups
	}
	if c.LogRotation.MaxDayAge == 0 {
		c.LogRotation.MaxDayAge = defMaxDayAge
	}
	if c.LogRotation.MaxMBSize == 0 {
		c.LogRotation.MaxMBSize = defMaxMBSize
	}

	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()

	return nil
}

// Save writes config back to JSON file
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, err := json.MarshalIndent(m.cfg, "", "	")
	if err != nil {
		return err
	}
	tmp := m.FileName + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, m.FileName)
}

// Reload reloads config from file
func (m *Manager) Reload() error {
	b, err := os.ReadFile(m.FileName)
	if err != nil {
		return err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()
	return nil
}

// Get returns a copy of config (thread-safe)
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) SetLastProduction(productionID int) error {
	return m.Update(func(c *Config) {
		c.Production.LastProduction = productionID
	})
}

// RemoveFromUnclosedProductions removes newly closed productions from UnclosedProductions.
func (m *Manager) RemoveFromUnclosedProductions(closedProductions []int) error {
	return m.Update(func(c *Config) {
		if c.Production.UnclosedProductions == nil {
			return
		}
		for _, k := range closedProductions {
			delete(c.Production.UnclosedProductions, k)
		}
	})
}

// AddUnclosedProduction adds new unclosed producition.
func (m *Manager) AddUnclosedProduction(unclosedProductionID int, productionDate time.Time) error {
	return m.Update(func(c *Config) {
		if c.Production.UnclosedProductions == nil {
			c.Production.UnclosedProductions = make(UnclosedProductionList)
		}
		c.Production.UnclosedProductions[unclosedProductionID] = productionDate
	})
}

// Update modifies config with provided function, then saves
func (m *Manager) Update(fn func(*Config)) error {
	m.mu.Lock()
	fn(&m.cfg)
	m.mu.Unlock()
	return m.Save()
}

func (m *Manager) QueryInterval() time.Duration {
	c := m.Get()
	if c.QueryIntervalS <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.QueryIntervalS) * time.Second
}

func (m *Manager) ProductionErrorDebounce() time.Duration {
	c := m.Get()
	if c.EmptyDebounceS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ProductionErrorDebounceS) * time.Second
}

func (m *Manager) EmptyDebounce() time.Duration {
	c := m.Get()
	if c.EmptyDebounceS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.EmptyDebounceS) * time.Second
}

func (m *Manager) ConnectRetry() time.Duration {
	c := m.Get()
	if c.ConnectRetryS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ConnectRetryS) * time.Second
}

func (m *Manager) QueryProductionCount() int {
	c := m.Get()
	if c.Production.QueryProductionCount <= 0 {
		return 1
	}
	return c.Production.QueryProductionCount
}

func (m *Manager) CheckUnclosedProductions() time.Duration {
	c := m.Get()
	if c.Production.CheckUnclosedProductionsH <= 0 {
		return 1 * time.Hour
	}
	return time.Duration(c.Production.CheckUnclosedProductionsH) * time.Hour
}

func (m *Manager) ShutdownTimeout() time.Duration {
	c := m.Get()
	if c.ShutdownS <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.ShutdownS) * time.Second
}

func (m *Manager) Config() Config {
	return m.cfg
}
