package network

import (
	"hivpn/log"
	"sync"

	"github.com/fasthttp/websocket"
)

type ARPRecord struct {
	Conn *websocket.Conn
	Key  []byte
}

type ARP struct {
	mu    sync.Mutex
	Table map[string]ARPRecord
}

func NewARP() (arp *ARP) {
	arp = new(ARP)
	arp.mu.Lock()
	defer arp.mu.Unlock()
	arp.Table = make(map[string]ARPRecord, 0)
	return
}

func (arp *ARP) QueryOne(ip string) ARPRecord {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	for _, v := range arp.Table {
		return v
	}

	return ARPRecord{}
}

func (arp *ARP) IsExist(ip string) bool {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	_, found := arp.Table[ip]
	return found
}

func (arp *ARP) Query(ip string) ARPRecord {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	conn, found := arp.Table[ip]
	if !found {
		return ARPRecord{}
	}

	return conn
}

func (arp *ARP) Delete(id string) {
	if len(id) < 1 {
		return
	}
	arp.mu.Lock()
	defer arp.mu.Unlock()
	log.Debug("arptable remove", id)
	delete(arp.Table, id)
}

func (arp *ARP) Update(id string, conn *websocket.Conn, key []byte) {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	current, found := arp.Table[id]
	if found {
		current.Conn.WriteMessage(websocket.TextMessage, []byte("You have logged in at another location"))
		current.Conn.Close()
	}

	arp.Table[id] = ARPRecord{conn, key}
}
