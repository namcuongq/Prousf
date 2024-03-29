package vpn

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"prousf/crypto"
	"prousf/log"
	"prousf/network"
	"prousf/tun"
	"prousf/utils"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fasthttp/websocket"
)

type Config struct {
	MTU            int
	TTL            time.Duration
	ServerAddr     string
	LocalAddr      string
	HostHeader     string
	DefaultGateway string
	IsServer       bool
	Whitelist      []string
	Blacklist      []string
	Users          []User
	Incognito      bool

	SSL    bool
	SSLKey string
	SSLCrt string

	RedirectGateway string
}

type User struct {
	Name string
	Pass string
	IP   string
}

type VPN struct {
	conf Config

	dev       tun.Device
	arpTable  *network.ARP
	userTable map[string]User
	blackList map[string]bool
	myNetwork *net.IPNet
	myIP      net.IP
	tryNumber int

	inMyNetwork func(ip net.IP) bool
	checkUpdate func(string, string, string) string
	queryArp    func(ip string) (network.ARPRecord, bool)
}

const (
	TUN_NAME = "MyNIC"

	TIME_TO_TRY = 5 * time.Second
	MAX_TRY     = 10

	WEBSOCKET_PATH              = "/home"
	VERSION_PATH                = "/version"
	AUTHEN_HEADER               = "Cookie"
	USERAGENT                   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.3"
	ERROR_AUTHENTICATION_FAILED = "Authentication failed"
	ERROR_LOGGED_ANOTHER        = "You have logged in at another location"

	VERSION = "2.0.3"
	RELEASE = "(04/05/2023)"
)

var (
	YOUR_OS = runtime.GOOS
)

func Create(conf Config) (vpn *VPN, err error) {
	vpn = new(VPN)
	vpn.conf = conf
	vpn.blackList = make(map[string]bool, 0)
	vpn.myIP, vpn.myNetwork, err = net.ParseCIDR(vpn.conf.LocalAddr)
	if err != nil {
		return
	}

	log.Debug("Create Virtual Network Adapter")
	vpn.dev, err = tun.CreateTUN(TUN_NAME, vpn.conf.MTU)
	if err != nil {
		return
	}
	defer vpn.stop()

	log.Debug("Make ARP Table")
	vpn.arpTable = network.NewARP()

	log.Debug("Setup Authentication")
	vpn.setupAuthentication()
	vpn.handlerCtrC()
	vpn.captureDev()

	if vpn.conf.IsServer { //is server mode
		vpn.queryArp = vpn.arpTable.Query
		vpn.inMyNetwork = func(ip net.IP) bool {
			return vpn.myNetwork.Contains(ip)
		}

		vpn.startServer()
	} else { // is client mode
		again := false
		vpn.queryArp = vpn.arpTable.QueryOne

		for {
			vpn.tryNumber++
			if vpn.tryNumber >= MAX_TRY {
				break
			}

			vpn.startClient(again)
			again = true
			vpn.arpTable.Delete(vpn.myIP.String())
			log.Info(fmt.Sprintf("Try again(%d/%d) in ", vpn.tryNumber+1, MAX_TRY), TIME_TO_TRY, "...")
			if vpn.tryNumber > 0 {
				time.Sleep(TIME_TO_TRY)
			}

		}
	}
	return
}

func (vpn *VPN) startServer() {
	var upgrader = websocket.Upgrader{}

	handlerClient := func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(AUTHEN_HEADER)
		idRequest, key := vpn.authenConn(token)

		if len(idRequest) < 1 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(ERROR_AUTHENTICATION_FAILED))
			log.Debug(r.RemoteAddr, ERROR_AUTHENTICATION_FAILED)
			return
		}

		if vpn.arpTable.IsExist(idRequest) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(ERROR_LOGGED_ANOTHER))
			log.Debug(idRequest, ERROR_LOGGED_ANOTHER)
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("Upgrade socket error:", err)
			return
		}
		defer func() {
			c.Close()
			vpn.arpTable.Delete(idRequest)
			log.Debug("close client", idRequest)
		}()

		arpData, _ := vpn.arpTable.Update(idRequest, key)

		go vpn.devToTun(arpData, c)
		vpn.tunToDev(key, c)
	}

	http.HandleFunc(WEBSOCKET_PATH, handlerClient)
	http.HandleFunc(VERSION_PATH, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(VERSION))
	})
	log.Debug("Route Network")
	err := vpn.setupRoute()
	if err != nil {
		panic(err)
	}
	log.Info("VPN Server started successfully!")
	log.Info("Version:", VERSION, "-", RELEASE)
	if vpn.conf.SSL {
		log.Info("Listen:", vpn.conf.ServerAddr, "- SSL")
		log.Error(http.ListenAndServeTLS(vpn.conf.ServerAddr, vpn.conf.SSLCrt, vpn.conf.SSLKey, nil))
	} else {
		log.Info("Listen:", vpn.conf.ServerAddr, "- No SSL")
		log.Error(http.ListenAndServe(vpn.conf.ServerAddr, nil))
	}

}

func (vpn *VPN) startClient(again bool) {
	var keyByte = []byte(utils.GenUUID())
	var tokenUser string
	for k, v := range vpn.userTable {
		tokenByte, err := crypto.AESEncrypt([]byte(v.Pass), keyByte)
		if err != nil {
			log.Error("encrypt key error", err)
			return
		}
		tokenUser = k + ":" + base64.StdEncoding.EncodeToString(tokenByte)
		break
	}
	log.Debug("Your token:", tokenUser)

	vpn.inMyNetwork = func(ip net.IP) bool {
		return false
	}
	vpn.checkUpdate = utils.CheckUpdate

	var c *websocket.Conn
	var resp *http.Response
	scheme := "ws"
	if vpn.conf.SSL {
		scheme = "wss"
	}
	u := url.URL{Scheme: scheme, Host: vpn.conf.ServerAddr, Path: WEBSOCKET_PATH}

	headerReq := http.Header{
		AUTHEN_HEADER: []string{tokenUser},
		"User-Agent":  []string{USERAGENT},
	}

	if len(vpn.conf.HostHeader) > 0 {
		headerReq["Host"] = []string{vpn.conf.HostHeader}
	}

	dialer := websocket.Dialer{}
	if vpn.conf.SSL {
		caCert, err := ioutil.ReadFile(vpn.conf.SSLCrt)
		if err != nil {
			log.Error("Error opening cert file "+vpn.conf.SSLCrt+", error:", err)
			return
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig := &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}

		dialer = websocket.Dialer{
			TLSClientConfig: tlsConfig,
		}
	}

	c, resp, err := dialer.Dial(u.String(), headerReq)
	if err != nil {
		var b []byte
		if resp != nil {
			defer func() {
				resp.Body.Close()
			}()
			b, _ = io.ReadAll(resp.Body)
		}
		log.Error(fmt.Sprintf("dial %s error: %s \n%s", u.String(), err.Error(), string(b)))
		return
	}

	defer func() {
		c.Close()
	}()

	if !again {
		log.Debug("Route Network")
		err = vpn.setupRoute()
		if err != nil {
			log.Error("setup route error", err)
			return
		}
	}

	log.Info("VPN Client started successfully!")
	log.Info("Version:", VERSION, "-", RELEASE)
	vpn.tryNumber = 0
	// if !again {
	// scheme := "http://"
	// if vpn.conf.SSL {
	// 	scheme = "https://"
	// }
	// fmt.Print(vpn.checkUpdate(scheme+vpn.conf.ServerAddr+VERSION_PATH, VERSION, vpn.conf.HostHeader))
	// }

	arpData, _ := vpn.arpTable.Update(vpn.myIP.String(), keyByte)
	go vpn.devToTun(arpData, c)
	vpn.tunToDev(keyByte, c)
}

func (vpn *VPN) tunToDev(key []byte, c *websocket.Conn) {
	for {
		c.SetReadDeadline(time.Now().Add(vpn.conf.TTL * 4 / 3))
		messType, message, err := c.ReadMessage()
		if err != nil {
			log.Error("read message from tun error:", err)
			return
		}

		switch messType {
		case websocket.TextMessage:
		default:
			rawData, err := crypto.AESDecrypt(key, message)
			if err != nil {
				log.Debug("decrypt data error", err)
				return
			}

			// header := network.ParseHeaderPacket(rawData)
			// if vpn.conf.IsServer {
			// 	if !vpn.myIP.Equal(header.IPDst) && vpn.arpTable.IsExist(header.IPDst.String()) {
			// 		arp := vpn.arpTable.Query(header.IPDst.String())
			// 		enData, err := crypto.AESEncrypt(arp.Key, rawData)
			// 		if err != nil {
			// 			log.Debug("tun to tun encrypt data error", err)
			// 			continue
			// 		}

			// 		err = arp.Conn.WriteMessage(websocket.BinaryMessage, enData)
			// 		if err != nil {
			// 			log.Debug("write tun to tun error", err)
			// 		}
			// 		continue
			// 	}
			// } else {
			// 	if vpn.myIP.Equal(header.IPDst) && vpn.conf.Incognito {
			// 		continue
			// 	}
			// }

			_, err = vpn.dev.Write(rawData, 0)
			if err != nil {
				log.Debug("write tun to dev error", err)
				return
			}
		}
	}
}

func (vpn *VPN) captureDev() {
	buf := make([]byte, vpn.conf.MTU)
	go func() {
		for {
			n, err := vpn.dev.Read(buf, 0)
			if err != nil {
				log.Error("read data from vpn error", err)
				break
			}

			if n > vpn.conf.MTU {
				log.Info("read large data", n, vpn.conf.MTU)
				n = vpn.conf.MTU
			}
			packet := buf[:n]

			header := network.ParseHeaderPacket(packet)
			if vpn.blackList[header.IPDst.String()] {
				log.Debug("Block ip", header.IPDst)
				continue
			}

			c, ok := vpn.queryArp(header.IPDst.String())
			if !ok {
				continue
			}

			dataEn, err := crypto.AESEncrypt(c.Key, packet)
			if err != nil {
				log.Debug("encrypt data error", err)
				continue
			}

			c.Conn <- dataEn

		}
	}()
}

func (vpn *VPN) devToTun(arpData network.ARPRecord, c *websocket.Conn) {
	ticker := time.NewTicker(vpn.conf.TTL)
	defer func() {
		log.Debug("quit dev to tun", c.LocalAddr(), c.RemoteAddr())
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-arpData.Conn:
			if !ok {
				log.Debug("close dev to tun", c.LocalAddr(), c.RemoteAddr())
				return
			}

			err := c.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Debug("write dev to tun error", err)
				return
			}
		case <-ticker.C:
			log.Trace("send ping", c.RemoteAddr())
			err := c.WriteMessage(websocket.TextMessage, []byte("ping"))
			if err != nil {
				log.Debug("send ping error", err)
				return
			}
		}

	}
}

func (vpn *VPN) handlerCtrC() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		vpn.stop()
		os.Exit(1)
	}()
}

func (vpn *VPN) authenConn(token string) (string, []byte) {
	arr := strings.Split(token, ":")
	if len(arr) < 2 {
		return "", nil
	}
	user := arr[0]
	u, found := vpn.userTable[user]
	if !found {
		return "", nil
	}
	keyBase64 := arr[1]

	tokenByte, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", nil
	}

	keyByte, err := crypto.AESDecrypt([]byte(u.Pass), tokenByte)
	if err != nil {
		return "", nil
	}

	for _, c := range keyByte {
		if c < 48 || (58 < c && c < 64) || (91 < c && c < 96) || c > 123 {
			return "", nil
		}
	}

	return u.IP, keyByte
}

func (vpn *VPN) setupAuthentication() {
	KEY_LEN := 32
	vpn.userTable = make(map[string]User, 0)

	for _, u := range vpn.conf.Users {
		pass := ""
		if len(u.Pass) < KEY_LEN {
			pass = fmt.Sprintf("%s%s", u.Pass, strings.Repeat("t", KEY_LEN-len(u.Pass)))
		}

		vpn.userTable[u.Name] = User{
			Pass: pass,
			IP:   network.GetIp(u.IP),
		}
	}
}

func (vpn *VPN) setupRoute() error {
	if YOUR_OS == "linux" {
		tunCmd := [][]string{
			{"link", "set", "dev", TUN_NAME, "mtu", fmt.Sprintf("%d", vpn.conf.MTU)},
			{"addr", "add", vpn.conf.LocalAddr, "dev", TUN_NAME},
			{"link", "set", "dev", TUN_NAME, "up"},
		}

		if !vpn.conf.IsServer {
			tunCmd = append(tunCmd, [][]string{
				{"route", "add", "0.0.0.0/1", "dev", TUN_NAME},
				{"route", "add", "128.0.0.0/1", "dev", TUN_NAME},
			}...)
		}

		for _, cmdAgrs := range tunCmd {
			err := runCmd("/sbin/ip", cmdAgrs...)
			if err != nil {
				return err
			}
		}
	} else if YOUR_OS == "windows" && !vpn.conf.IsServer {
		currentDefaultGateway, err := network.GetDefaultGatewayWindows()
		if err != nil {
			return err
		}

		iface, err := net.InterfaceByName(TUN_NAME)
		if err != nil {
			return err
		}

		vpn.conf.Whitelist = append(vpn.conf.Whitelist, network.GetIp(vpn.conf.ServerAddr)+"/32")

		tunCmd := [][]string{
			{"netsh", "interface", "ip", "set", "address", fmt.Sprintf("name=%d", iface.Index), "source=static", "addr=" + network.GetIp(vpn.conf.LocalAddr), "mask=" + network.CIDRToMask(vpn.conf.LocalAddr), "gateway=none"},
			{"route", "add", network.GetIp(vpn.conf.RedirectGateway), "mask", network.CIDRToMask(vpn.conf.RedirectGateway), vpn.conf.DefaultGateway, "if", fmt.Sprintf("%d", iface.Index), "metric", "5"},
			// {"route", "add", "0.0.0.0", "mask", "0.0.0.0", vpn.conf.DefaultGateway, "if", fmt.Sprintf("%d", iface.Index), "metric", "5"},
			// {"route", "add", network.GetIp(vpn.conf.ServerAddr), "mask", "255.255.255.255", currentDefaultGateway.Gateway},
		}

		for _, ipW := range vpn.conf.Whitelist {
			tunCmd = append(tunCmd, []string{
				"route", "add", network.GetIp(ipW), "mask", network.CIDRToMask(ipW), currentDefaultGateway.Gateway,
			})
		}

		for _, ipB := range vpn.conf.Blacklist {
			tunCmd = append(tunCmd, []string{
				"route", "add", ipB, "mask", "255.255.255.255", vpn.conf.DefaultGateway, "if", fmt.Sprintf("%d", iface.Index), "metric", "5",
			})
			vpn.blackList[ipB] = true
		}

		for _, cmdAgrs := range tunCmd {
			err := runCmd(cmdAgrs[0], cmdAgrs[1:]...)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("not support os: %v", YOUR_OS)
	}

	return nil
}

func (vpn *VPN) stop() {
	log.Info("Stop vpn ...")
	if vpn.conf.IsServer {
	} else {
		if YOUR_OS == "linux" {

		} else if YOUR_OS == "windows" {
			for _, ipW := range vpn.conf.Whitelist {
				err := runCmd("route", "delete", network.GetIp(ipW), "mask", network.CIDRToMask(ipW))
				if err != nil {
					log.Error(err)
				}
			}

			for _, ipB := range vpn.conf.Blacklist {
				err := runCmd("route", "delete", ipB)
				if err != nil {
					log.Error(err)
				}
			}
			// vpn.dev.Close()
		}
	}
	log.Info("Done!(GoodBye)")
	// fmt.Println("Press the Enter Key to exit!")
	// fmt.Scanln()
}

func runCmd(c string, args ...string) error {
	log.Debug(c, strings.Join(args, " "))
	b := new(strings.Builder)
	cmd := exec.Command(c, args...)
	cmd.Stdout = b
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("run cmd error: %v", err)
	}

	if strings.TrimSpace(b.String()) != "OK!" && strings.TrimSpace(b.String()) != "" {
		fmt.Println(b.String())
	}
	return err
}
