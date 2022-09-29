package connection

type TUN struct {
	Addr              string
	HostHeader        string
	Run               func() error
	FuncWriteTunToDev func(key, data []byte)
	FuncWriteDevToTun func(conn interface{}, data []byte) error
	FuncAuthenConn    func(token string, conn interface{}) (string, []byte, func(id string))
}

const (
	CONNECTION_TYPE_WEBSOCKET   = 1
	ERROR_AUTHENTICATION_FAILER = "Authentication failed"
)

func (self *TUN) Connect(token string, connectType int) error {
	switch connectType {
	case CONNECTION_TYPE_WEBSOCKET:
		srcConn, runFunc, err := self.createWebSocket(self.Addr, token)
		if err != nil {
			return err
		}

		srcConn.OnFuncWriteTunToDev(self.FuncWriteTunToDev)
		srcConn.OnAuthen(self.FuncAuthenConn)
		self.FuncWriteDevToTun = srcConn.WriteDevToTun

		self.onRun(runFunc)
	default:
	}
	return nil
}

func (t *TUN) onRun(f func() error) {
	t.Run = f
}
