package main

import "github.com/dronm/elkonexport/logger"

func (a *App) Login() {
	cfg := a.cfgMgr.Get()
	// retry Login until success or quit
	for {
		select {
		case <-a.ctx.Done():
			return 
		default:
		}

		token, err := Login(cfg.Auth.URL)
		if err != nil {
			logger.Logger.Errorf("Login to application server failed: %v; retrying...", err)
			if !waitOrCancelled(a.ctx, a.cfgMgr.ConnectRetry()) {
				return 
			}
			continue
		}
		a.authToken = token
		break
	}
	logger.Logger.Info("successfully logged to application server")
}
