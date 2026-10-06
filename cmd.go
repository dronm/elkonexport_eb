package main

import (
	"errors"
	"strings"
	"time"

	"github.com/dronm/elkonexport/config"
	"github.com/dronm/elkonexport/logger"
)

type Command struct {
	Cmd     string         `json:"cmd"`
	QueryID string         `json:"query_id"`
	Params  map[string]any `json:"params,omitempty"`
}

func (a *App) commandHandler() {
	for {
		logger.Logger.Debug("Waiting for new command...") 

		select {
		case <-a.ctx.Done():
			logger.Logger.Info("commandHandler: context done, exiting...")
			return
		case cmd := <-a.cmdCh:
			logger.Logger.Debugf("commandHandler: Processing command: %v", cmd)
			a.handleCommand(cmd)
		case <-a.reloadCfgCh:
			_ = a.cfgMgr.Reload()
			logger.Logger.Info("config reloaded")
		}
	}
}

func (a *App) handleCommand(cmd Command) {
	switch strings.ToLower(cmd.Cmd) {
	case "get":
		cfg := a.cfgMgr.Get()
		v, ok := cmd.Params["param"]
		if !ok {
			err := errors.New("command get, param not defined")
			logger.Logger.Errorf("%v", err)
			sendError(a, err, cmd.QueryID)
		}
		if f, ok := v.(string); ok {
			switch f {
			case "production_site_id":
				sendPayload(a, "Get", cfg.Production.ProductionSiteID, cmd.QueryID)
			case "production_site_descr":
				sendPayload(a, "Get", cfg.Production.ProductionSiteDescr, cmd.QueryID)
			case "last_production":
				sendPayload(a, "Get", cfg.Production.LastProduction, cmd.QueryID)
			case "unclosed_productions":
				sendPayload(a, "Get", cfg.Production.UnclosedProductions, cmd.QueryID)
			case "websocket_url":
				sendPayload(a, "Get", cfg.WebSocketURL, cmd.QueryID)
			case "mssql_conn_str":
				sendPayload(a, "Get", cfg.MSSQLConnStr, cmd.QueryID)
			case "query_interval_s":
				sendPayload(a, "Get", cfg.QueryIntervalS, cmd.QueryID)
			case "empty_debounce_s":
				sendPayload(a, "Get", cfg.EmptyDebounceS, cmd.QueryID)
			case "query_production_count":
				sendPayload(a, "Get", cfg.Production.QueryProductionCount, cmd.QueryID)
			case "log_level":
				sendPayload(a, "Get", cfg.LogLevel, cmd.QueryID)
			}
		}
	case "set":
		err := a.cfgMgr.Update(func(c *config.Config) {
			for k, v := range cmd.Params {
				switch strings.ToLower(k) {
				case "websocket_url":
					if f, ok := v.(string); ok {
						c.WebSocketURL = f
					} else {
						sendError(a, errors.New("parameter not a string"), cmd.QueryID)
					}
				case "mssql_conn_str":
					if f, ok := v.(string); ok {
						c.MSSQLConnStr = f
					} else {
						sendError(a, errors.New("parameter not a string"), cmd.QueryID)
					}
				case "query_interval_s":
					if f, ok := toInt(v); ok {
						c.QueryIntervalS = f
					} else {
						sendError(a, errors.New("parameter not an int"), cmd.QueryID)
					}
				case "empty_debounce_s":
					if f, ok := toInt(v); ok {
						c.EmptyDebounceS = f
					} else {
						sendError(a, errors.New("parameter not an int"), cmd.QueryID)
					}
				case "shutdown_s":
					if f, ok := toInt(v); ok {
						c.ShutdownS = f
					} else {
						sendError(a, errors.New("parameter not an int"), cmd.QueryID)
					}
				case "query_production_count":
					if f, ok := toInt(v); ok {
						c.Production.QueryProductionCount = f
					} else {
						sendError(a, errors.New("parameter not an int"), cmd.QueryID)
					}
				case "log_level":
					if f, ok := v.(string); ok {
						c.LogLevel = f
						logger.SetLogLevel(f)
					} else {
						sendError(a, errors.New("parameter not a string"), cmd.QueryID)
					}
				case "log_file":
					if f, ok := v.(string); ok {
						c.LogFileName = f
					} else {
						sendError(a, errors.New("parameter not a string"), cmd.QueryID)
					}
				case "last_production":
					if f, ok := toInt(v); ok {
						c.Production.LastProduction = f
					} else {
						sendError(a, errors.New("parameter not an int"), cmd.QueryID)
					}
				}
			}
		})
		if err == nil {
			logger.Logger.Info("config updated from command")
		}
	case "fetch_production":
		if v, ok := cmd.Params["production_id"]; ok {
			if f, ok := toInt(v); ok {
				if err := a.cfgMgr.AddUnclosedProduction(f, time.Now()); err != nil {
					logger.Logger.Errorf("AddUnclosedProduction(): %s", err)
					sendError(a, err, cmd.QueryID)
				}
			} else {
				sendError(a, errors.New("parameter not an int"), cmd.QueryID)
			}
		} else {
			sendError(a, errors.New("parameter production_id is missing"), cmd.QueryID)
		}
	case "fetch_config":
		cfg := a.cfgMgr.Config()
		sendPayload(a, "FetchConfig", cfg, cmd.QueryID)

	case "set_last_production":
		if v, ok := cmd.Params["production_id"]; ok {
			if f, ok := toInt(v); ok {
				if err := a.cfgMgr.SetLastProduction(f); err != nil {
					logger.Logger.Errorf("SetLastProduction(): %s", err)
					sendError(a, err, cmd.QueryID)
				}
			} else {
				logger.Logger.Error("parameter not an int")
				sendError(a, errors.New("parameter not an int"), cmd.QueryID)
			}
		} else {
			logger.Logger.Error("parameter production_id is missing")
			sendError(a, errors.New("parameter production_id is missing"), cmd.QueryID)
		}
	case "fetch_log":
		if v, ok := cmd.Params["count"]; ok {
			if f, ok := toInt(v); ok {
				fileName := a.cfgMgr.Get().LogFileName
				res, err := tailFile(fileName, f)
				if err != nil {
					logger.Logger.Errorf("tailFile(): %s", err)
					sendError(a, err, cmd.QueryID)
				} else {
					sendPayload(a, "Log", res, cmd.QueryID)
				}
			} else {
				sendError(a, errors.New("parameter not an int"), cmd.QueryID)
			}
		}
	case "save":
		if err := a.cfgMgr.Save(); err != nil {
			logger.Logger.Errorf("Save(): %s", err)
			sendError(a, err, cmd.QueryID)
		}
	case "reload":
		if err := a.cfgMgr.Reload(); err != nil {
			logger.Logger.Errorf("Reload(): %s", err)
			sendError(a, err, cmd.QueryID)
		}
	case "stop":
		a.Shutdown()
	}
}

func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	}
	return 0, false
}
