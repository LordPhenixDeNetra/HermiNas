package wshub

import (
	"net/http"

	"github.com/gorilla/websocket"

	"herminas/engine/auth"
)

// ServeWS upgrades an HTTP request to a WebSocket connection, registers
// the resulting client with hub, and starts its read/write pumps.
//
// Browsers' native WebSocket API can't set an Authorization header, so
// unlike every other authenticated route in engine/api (M1.5's
// auth.Authenticate middleware), the session JWT travels as a `?token=`
// query parameter here instead — a deliberate, narrower exception to the
// header-bearer convention, not a bypass of it: the token is still the
// same short-lived JWT, still verified with the same JWTManager, before
// the connection is ever upgraded.
//
// devCORS mirrors api.Config.DevCORS (M1.5): relaxed origin checking only
// for the Vite dev server talking to the API on a different port.
func ServeWS(hub *Hub, jwtMgr *auth.JWTManager, runner QueryRunner, devCORS bool) http.HandlerFunc {
	upgrader := websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096}
	if devCORS {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := jwtMgr.Verify(r.URL.Query().Get("token"))
		if err != nil {
			http.Error(w, "invalid or missing token", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade already wrote an HTTP error response
		}

		c := newClient(hub, conn, runner, claims.Username)
		hub.add(c)
		go c.writePump()
		go c.readPump()
	}
}
