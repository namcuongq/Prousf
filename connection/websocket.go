package connection

import (
	"hivpn/log"
	"net/http"
	"net/url"

	"github.com/fasthttp/websocket"
)

const (
	WEBSOCKET_PATH = "/tunnel"
	AUTHEN_HEADER  = "User"
)

var upgrader = websocket.Upgrader{}

type tunWebsocket struct {
	writeTunToDev func(key, data []byte)
	authen        func(id string, conn interface{}) (string, []byte, func(id string))
}

func (self *tunWebsocket) OnFuncWriteTunToDev(f func(key, data []byte)) {
	self.writeTunToDev = f
}

func (self *tunWebsocket) WriteDevToTun(conn interface{}, data []byte) error {
	return conn.(*websocket.Conn).WriteMessage(websocket.BinaryMessage, data)
}

func (self *tunWebsocket) OnAuthen(f func(id string, conn interface{}) (string, []byte, func(id string))) {
	self.authen = f
}

func (t *tunWebsocket) handlerClient(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	token := r.Header.Get(AUTHEN_HEADER)
	idRequest, key, cancel := t.authen(token, c)

	if len(idRequest) < 1 {
		return
	}

	for {
		_, frame, err := c.ReadMessage()
		if err != nil {
			log.Debug(err)
			break
		}

		t.writeTunToDev(key, frame)
	}
	cancel(idRequest)
}

func (t *tunWebsocket) handlerServer(token string, c *websocket.Conn) {
	defer c.Close()
	idReq, key, cancel := t.authen(token, c)
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Error("Authentication failed please try again...!")
			break
		}

		t.writeTunToDev(key, message)
	}
	cancel(idReq)
}

func (t *TUN) createWebSocket(addr string, token string) (newTun *tunWebsocket, runFunc func() error, err error) {
	newTun = new(tunWebsocket)
	if token == "" {
		http.HandleFunc(WEBSOCKET_PATH, newTun.handlerClient)

		runFunc = func() error {
			return http.ListenAndServe(addr, nil)
		}
	} else {
		var c *websocket.Conn
		u := url.URL{Scheme: "ws", Host: addr, Path: WEBSOCKET_PATH}
		c, _, err = websocket.DefaultDialer.Dial(u.String(), http.Header{
			AUTHEN_HEADER: []string{token},
		})
		if err != nil {
			return
		}

		runFunc = func() error {
			newTun.handlerServer(token, c)
			return nil
		}

	}

	return
}
