package connection

import (
	"fmt"
	"hivpn/log"
	"io"
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
		log.Debug("Upgrade socket error:", err)
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
			log.Debug("read message err:", err)
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
			log.Error("Authentication failed Or Cannot connect to the server !", err)
			break
		}

		t.writeTunToDev(key, message)
	}
	cancel(idReq)
}

func (t *TUN) createWebSocket(addr, token string) (newTun *tunWebsocket, runFunc func() error, err error) {
	newTun = new(tunWebsocket)
	if token == "" {
		http.HandleFunc(WEBSOCKET_PATH, newTun.handlerClient)

		runFunc = func() error {
			return http.ListenAndServe(addr, nil)
		}
	} else {
		var c *websocket.Conn
		var resp *http.Response
		u := url.URL{Scheme: "ws", Host: addr, Path: WEBSOCKET_PATH}

		headerReq := http.Header{
			AUTHEN_HEADER: []string{token},
		}

		if len(t.HostHeader) > 0 {
			headerReq["Host"] = []string{t.HostHeader}
		}

		c, resp, err = websocket.DefaultDialer.Dial(u.String(), headerReq)
		if err != nil {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			err = fmt.Errorf("dail %s error: %s \n%v\n%s", u.String(), err.Error(), resp.Header, string(b))
			return
		}

		runFunc = func() error {
			newTun.handlerServer(token, c)
			return nil
		}

	}

	return
}
