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
	arp.Table = make(map[string]ARPRecord, 0)
	return
}

func (arp *ARP) QueryOne(ip string) (ARPRecord, bool) {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	for _, v := range arp.Table {
		return v, true
	}

	return ARPRecord{}, false
}

func (arp *ARP) IsExist(ip string) bool {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	_, found := arp.Table[ip]
	return found
}

func (arp *ARP) Query(ip string) (ARPRecord, bool) {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	conn, found := arp.Table[ip]
	return conn, found
}

func (arp *ARP) Delete(id string) {
	if len(id) < 1 {
		return
	}
	arp.mu.Lock()
	defer arp.mu.Unlock()
	current, found := arp.Table[id]
	if found {
		log.Debug("arptable delete", id)
		delete(arp.Table, id)
		if len(current.Conn) > 1 {
			<-current.Conn
		}
		close(current.Conn)
	}
}

func (arp *ARP) Update(id string, key []byte) (ARPRecord, bool) {
	arp.mu.Lock()
	defer arp.mu.Unlock()
	_, found := arp.Table[id]
	if found {
		// 	// current.Conn.Close()
		// close(current.Conn)
		return ARPRecord{}, found
	}
	conn := make(chan []byte, 100)
	newData := ARPRecord{conn, key}
	arp.Table[id] = newData
	listClient := []string{}
	for c, _ := range arp.Table {
		listClient = append(listClient, c)
	}
	log.Debug("update arptable", listClient)
	return newData, false
}
