package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dronm/elkonexport/config"
	"github.com/dronm/elkonexport/logger"

	"github.com/gorilla/websocket"
)

type App struct {
	productionSiteID int // constant

	cfgMgr *config.Manager

	authToken string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	db     *sql.DB
	wsConn *websocket.Conn

	cmdCh        chan Command
	saveCfgCh    chan struct{}
	reloadCfgCh  chan struct{}
	sendResultCh chan any
	// quitCh       chan struct{}

	wsMu sync.Mutex // protects wsConn + reconnect
}

// NewApp creates a new App instance
func NewApp(cfgMgr *config.Manager) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		productionSiteID: cfgMgr.Get().Production.ProductionSiteID,
		cfgMgr:           cfgMgr,
		ctx:              ctx,
		cancel:           cancel,
		cmdCh:            make(chan Command, 8),
		saveCfgCh:        make(chan struct{}, 2),
		reloadCfgCh:      make(chan struct{}, 2),
		sendResultCh:     make(chan any, 16),
		// quitCh:           make(chan struct{}),
	}

	return a, nil
}


// Run starts the application loops
func (a *App) Run() error {
	//infinite retry
	a.ConnectMSSQLWithRetry()
	a.Login()
	_ = a.reconnectWebSocket()

	// spawn goroutines
	a.wg.Add(1)
	go func() { defer a.wg.Done(); a.wsReadLoop() }()

	a.wg.Add(1)
	go func() { defer a.wg.Done(); a.wsWriterLoop() }()

	a.wg.Add(1)
	go func() { defer a.wg.Done(); a.commandHandler() }()

	a.wg.Add(1)
	go func() { defer a.wg.Done(); a.queryLoop() }()

	// wait until quit
	// <-a.quitCh
	<-a.ctx.Done()
	a.doShutdown()

	return nil
}

func waitOrCancelled(ctx context.Context, d time.Duration) bool {
    t := time.NewTimer(d)
    defer t.Stop()
    select {
    case <-ctx.Done():
        return false
    case <-t.C:
        return true
    }
}

// Shutdown requests a graceful shutdown
func (a *App) Shutdown() {
	a.cancel()
	// select {
	// case a.quitCh <- struct{}{}:
	// default:
	// }
}

func (a *App) doShutdown() {
	logger.Logger.Info("graceful shutdown starting")
	a.cancel()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	timeout := a.cfgMgr.ShutdownTimeout()
	select {
	case <-done:
		logger.Logger.Info("graceful shutdown complete")
	case <-time.After(timeout):
		logger.Logger.Warn("shutdown timeout exceeded")
	}

	if a.db != nil {
		_ = a.db.Close()
	}
	if a.wsConn != nil {
		_ = a.wsConn.Close()
	}
}

func (a *App) SendSync(v any) error {
	a.wg.Add(1)
	defer a.wg.Done()

	if a.wsConn == nil {
		return fmt.Errorf("wsConn is nil")
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Block until message is written
	return a.wsConn.WriteMessage(websocket.TextMessage, b)
}
