package network

import (
	"sync"
)

type ARPRecord struct {
	Conn interface{}
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
	arp.mu.Lock()
	defer arp.mu.Unlock()
	delete(arp.Table, id)
}

func (arp *ARP) Update(id string, conn interface{}, key []byte) bool {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	_, found := arp.Table[id]
	if found {
		return found
	}

	arp.Table[id] = ARPRecord{conn, key}
	return found
}
