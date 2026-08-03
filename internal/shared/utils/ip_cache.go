package utils

import (
	"sync"
	"time"
)

type IpEntry struct {
	Keywords string
	Attempt  int
}

var (
	mu        sync.Mutex
	cacheDay  int
	ipEntries = make(map[string]IpEntry)
)

func resetIfNewDay() {
	currentYD := time.Now().YearDay()
	if cacheDay != currentYD {
		cacheDay = currentYD
		ipEntries = make(map[string]IpEntry)
	}
}

func GetIPEntry(ip string) (IpEntry, bool) {
	mu.Lock()
	defer mu.Unlock()
	resetIfNewDay()
	entry, ok := ipEntries[ip]
	return entry, ok
}

func SetIPEntry(ip string, entry IpEntry) {
	mu.Lock()
	defer mu.Unlock()
	resetIfNewDay()
	ipEntries[ip] = entry
}
