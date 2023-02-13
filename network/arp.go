package network

import (
	"hivpn/log"
	"sync"
)

type ARPRecord struct {
	Conn chan []byte
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

func (arp *ARP) QueryOne(ip string) (ARPRecord, bool) {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	for _, v := range arp.Table {
		return v, true
	}

	return ARPRecord{}, false
}

func (arp *ARP) IsExist(ip string) bool {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	_, found := arp.Table[ip]
	return found
}

func (arp *ARP) Query(ip string) (ARPRecord, bool) {
	// arp.mu.Lock()
	// defer arp.mu.Unlock()
	conn, found := arp.Table[ip]
	return conn, found
}

func (arp *ARP) Delete(id string) {
	if len(id) < 1 {
		return
	}
	arp.mu.Lock()
	defer arp.mu.Unlock()
	delete(arp.Table, id)
	log.Debug("arptable", arp.Table)
}

func (arp *ARP) Update(id string, conn chan []byte, key []byte) {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	_, found := arp.Table[id]
	if found {
		// current.Conn.Close()
		// close(current.Conn)
	}
	arp.Table[id] = ARPRecord{conn, key}
	log.Debug("arptable", arp.Table)
}
