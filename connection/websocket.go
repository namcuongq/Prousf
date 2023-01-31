package connection

import (
	"fmt"
	"hivpn/log"
	"hivpn/network"
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
	authen        func(id string) (string, []byte)
	arpTable      *network.ARP
}

func (self *tunWebsocket) OnFuncWriteTunToDev(f func(key, data []byte)) {
	self.writeTunToDev = f
}

func (self *tunWebsocket) WriteDevToTun(conn interface{}, data []byte) error {
	return conn.(*websocket.Conn).WriteMessage(websocket.BinaryMessage, data)
}

func (self *tunWebsocket) OnAuthen(f func(id string) (string, []byte)) {
	self.authen = f
}

func (t *tunWebsocket) handlerClient(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get(AUTHEN_HEADER)
	idRequest, key := t.authen(token)

	if len(idRequest) < 1 {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(ERROR_AUTHENTICATION_FAILED))
		log.Debug(r.RemoteAddr, ERROR_AUTHENTICATION_FAILED)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("Upgrade socket error:", err)
		return
	}
	t.readMessage(c, idRequest, key, 1)
}

func (t *tunWebsocket) handlerServer(token string, c *websocket.Conn) {
	idReq, key := t.authen(token)
	t.readMessage(c, idReq, key, 0)
}

func (t *tunWebsocket) readMessage(c *websocket.Conn, idRequest string, key []byte, mode int) {
	defer func() {
		c.Close()
		t.arpTable.Delete(idRequest)
	}()

	t.arpTable.Update(idRequest, c, key)

	for {
		messType, frame, err := c.ReadMessage()
		if err != nil {
			log.Debug("read message err:", err)
			break
		}

		if messType == websocket.TextMessage {
			log.Info(string(frame))
			continue
		}

		t.writeTunToDev(key, frame)
	}
}

func (t *TUN) createWebSocket(addr, token string, arpTable *network.ARP) (newTun *tunWebsocket, runFunc func() error, err error) {
	newTun = new(tunWebsocket)
	newTun.arpTable = arpTable
	if token == "" {
		http.HandleFunc(WEBSOCKET_PATH, newTun.handlerClient)

		runFunc = func() error {
			log.Info("Server listening on", addr)
			return http.ListenAndServe(addr, nil)
		}
	} else {
		log.Info("Connecting to", addr, "...")
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
			var b []byte
			if resp != nil {
				defer func() {
					resp.Body.Close()
				}()
				b, _ = io.ReadAll(resp.Body)
			}
			err = fmt.Errorf("dial %s error: %s \n%s", u.String(), err.Error(), string(b))
			return
		}
		log.Info("Successful connection to the server", addr)
		runFunc = func() error {
			newTun.handlerServer(token, c)
			return nil
		}

	}

	return
}
