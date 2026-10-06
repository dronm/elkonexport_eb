package main

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

func (a *App) connectWebSocket(websocketURL *url.URL) (int, error) {
	// fmt.Println("authToken:",a.authToken)
    dialer := websocket.Dialer{}
	c, httpResp, err := dialer.Dial(websocketURL.String()+"/"+a.authToken, nil)
	if err != nil {
		if httpResp != nil {
			httpResp.Body.Close()
			return httpResp.StatusCode, err
		}
		return http.StatusInternalServerError, err
	}
	a.wsConn = c

	return httpResp.StatusCode, nil
	
}

