package main

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/dronm/elkonexport/logger"

	"github.com/gorilla/websocket"
)

func (a *App) wsReadLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		if a.wsConn == nil {
			logger.Logger.Warn("wsReadLoop: no connection, reconnecting...")
			if !a.reconnectWebSocket() {
				continue
			}
		}

		_, msg, err := a.wsConn.ReadMessage()
		if err != nil {
			logger.Logger.Errorf("ws ReadMessage(): %v", err)
			a.closeWebSocket()
			continue
		}

		// process message
		var cmd Command
		if err := json.Unmarshal(msg, &cmd); err != nil {
			logger.Logger.Errorf("ws wsReadLoop json.Unmarshal(): %v for command: %v", err, cmd)
			continue
		}
		a.cmdCh <- cmd
	}
}

func (a *App) wsWriterLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-a.sendResultCh:
			if a.wsConn == nil {
				continue // wait for reconnect
			}

			b, err := json.Marshal(msg)
			if err != nil {
				logger.Logger.Errorf("ws json.Marshal(): %v", err)
				continue
			}
			if err := a.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
				logger.Logger.Errorf("ws WriteMessage(): %v", err)
			}
		}
	}
}

func (a *App) closeWebSocket() {
	a.wsMu.Lock()
	defer a.wsMu.Unlock()

	if a.wsConn != nil {
		_ = a.wsConn.Close()
		a.wsConn = nil
	}
}

func (a *App) reconnectWebSocket() bool {
	retryInterval := a.cfgMgr.ConnectRetry()

	for {
		select {
		case <-a.ctx.Done():
			return false
		default:
		}

		a.wsMu.Lock()
		cfg := a.cfgMgr.Get()
		wsURL, err := url.Parse(cfg.WebSocketURL)
		if err != nil {
			a.wsMu.Unlock()
			logger.Logger.Errorf("Invalid WebSocketURL: %v; retrying...", err)
			if !waitOrCancelled(a.ctx, retryInterval) {
				return false
			}
			continue
		}

		httpRespCode, err := a.connectWebSocket(wsURL)
		if err == nil && httpRespCode == http.StatusSwitchingProtocols {
			a.wsMu.Unlock()
			logger.Logger.Info("successfully connected to websocket server")
			return true
		}
		a.wsMu.Unlock()

		if httpRespCode == http.StatusForbidden || httpRespCode == http.StatusUnauthorized {
			// relogin
			logger.Logger.Debugf("httpRespCode: %d, relogging...", httpRespCode)
			a.Login()
		}

		logger.Logger.Errorf("connectWebSocket: %v, httpCode: %d; retrying...", err, httpRespCode)
		if !waitOrCancelled(a.ctx, retryInterval) {
			return false
		}
	}
}
