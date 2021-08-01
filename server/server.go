package server

import (
	"github.com/fatedier/golib/net/mux"
	"net"
	"time"
)

type Watchdog struct {
	mux        *mux.Mux
	watchTable *map[string]net.Listener
}

const (
	connectionTimeout time.Duration = 10 * time.Second
)

func NewWatchdog() (watchdog *Watchdog, err error) {
	watchdog = &Watchdog{}
	return watchdog, nil
}
