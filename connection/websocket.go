package connection

import (
	"fmt"
	"hivpn/log"
	"hivpn/network"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/fasthttp/websocket"
)

const (
	WEBSOCKET_PATH = "/tunnel"
	VERSION_PATH   = "/version"
	AUTHEN_HEADER  = "User"
)

var (
	upgrader   = websocket.Upgrader{}
	pingPeriod = 20 * time.Second
)

type chanData struct {
	conn *websocket.Conn
	data []byte
}

type tunWebsocket struct {
	writeTunToDev func(key, data []byte)
	authen        func(id string) (string, []byte)
	arpTable      *network.ARP

	send     chan chanData
	stopchan chan bool
}

func (self *tunWebsocket) OnFuncWriteTunToDev(f func(key, data []byte)) {
	self.writeTunToDev = f
}

func (self *tunWebsocket) OnClose() {
	log.Debug("close websocket")
	close(self.stopchan)
	close(self.send)
}

func (self *tunWebsocket) WriteDevToTun(conn interface{}, data []byte) error {
	select {
	case _, ok := <-self.stopchan:
		if !ok {
			return fmt.Errorf("websocket client is closed")
		}
	default:
		self.send <- chanData{
			conn: conn.(*websocket.Conn),
			data: data,
		}
	}
	return nil
	// return conn.(*websocket.Conn).WriteMessage(websocket.BinaryMessage, data)
}

func (self *tunWebsocket) writePump(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	go func() {
		for {
			select {
			case message, ok := <-self.send:
				if !ok {
					return
				}
				err := message.conn.WriteMessage(websocket.BinaryMessage, message.data)
				if err != nil {
					log.Error("write data to websocket error", err)
				}
			case <-ticker.C:
				if conn != nil {
					log.Debug("websocket client send ping message")
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Error("websocket client send ping error", err)
						return
					}
				}
			}
		}
	}()
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

	// if t.arpTable.IsExist(idRequest) {
	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	w.Write([]byte("You have logged in at another location"))
	// 	log.Debug(idRequest, "You have logged in at another location")
	// 	return
	// }

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("Upgrade socket error:", err)
		return
	}

	c.SetPingHandler(func(appData string) error {
		log.Debug("recv", c.RemoteAddr().String(), "ping message")
		log.Debug("send pong message to", c.RemoteAddr().String())
		w, err := c.NextWriter(websocket.PongMessage)
		if err != nil {
			log.Error("websocker server send pong message err:", err)
			return nil
		}
		w.Write(nil)

		if err := w.Close(); err != nil {
			log.Error("websocker server send pong message err:", err)
			return nil
		}
		// err = c.WriteMessage(websocket.PongMessage, []byte("pong"))
		// if err != nil {
		// 	log.Error("websocker server send pong message err:", err)
		// }
		return nil
	})

	t.readMessage(c, idRequest, key)
}

func (t *tunWebsocket) handlerServer(token string, c *websocket.Conn) {
	idReq, key := t.authen(token)
	t.readMessage(c, idReq, key)
}

func (t *tunWebsocket) readMessage(c *websocket.Conn, idRequest string, key []byte) {
	defer func() {
		c.Close()
		// t.arpTable.Delete(idRequest)
	}()

	t.arpTable.Update(idRequest, c, key)

	for {
		_, frame, err := c.ReadMessage()
		if err != nil {
			log.Error("read message err:", err)
			break
		}

		// if messType == websocket.TextMessage {
		// 	log.Info(string(frame))
		// 	continue
		// }

		t.writeTunToDev(key, frame)
	}
}

func (t *TUN) createWebSocket(addr, token string, arpTable *network.ARP) (newTun *tunWebsocket, runFunc func() error, err error) {
	newTun = new(tunWebsocket)
	newTun.arpTable = arpTable
	newTun.send = make(chan chanData, 256)
	newTun.stopchan = make(chan bool, 1)
	if token == "" { //server mode
		http.HandleFunc(WEBSOCKET_PATH, newTun.handlerClient)
		http.HandleFunc(VERSION_PATH, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(log.VERSION))
		})

		newTun.writePump(nil)
		runFunc = func() error {
			log.Info("Server listening on", addr)
			return http.ListenAndServe(addr, nil)
		}
	} else { // client mode
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
		t.TryNumber = 1
		newTun.writePump(c)
		c.SetPongHandler(func(appData string) error {
			log.Debug("recv", c.RemoteAddr().String(), "pong message")
			return nil
		})
		runFunc = func() error {
			newTun.handlerServer(token, c)
			return nil
		}

	}

	return
}
