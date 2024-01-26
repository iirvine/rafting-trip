package main

import (
	"log/slog"
	"net"
)

type Config struct {
	NSServerAddr string
	EWServerAddr string
	NSButtonAddr *net.UDPAddr
	EWButtonAddr *net.UDPAddr
}

// Application manages network connections and passing messages between the controller and the network.
type Application struct {
	conf      Config
	ctl       *Controller
	nsConn    net.Conn
	ewConn    net.Conn
	nsBtnConn *net.UDPConn
	ewBtnConn *net.UDPConn
}

func NewApplication(conf Config) *Application {
	return &Application{
		conf: conf,
		ctl:  NewController(),
	}
}

func (a *Application) Run() {
	nsConn, err := net.Dial("udp", a.conf.NSServerAddr)
	if err != nil {
		panic(err)
	}
	a.nsConn = nsConn

	ewConn, err := net.Dial("udp", a.conf.EWServerAddr)
	if err != nil {
		panic(err)
	}
	a.ewConn = ewConn

	nsBtnConn, err := net.ListenUDP("udp", a.conf.NSButtonAddr)
	if err != nil {
		panic(err)
	}
	a.nsBtnConn = nsBtnConn

	ewBtnConn, err := net.ListenUDP("udp", a.conf.EWButtonAddr)
	if err != nil {
		panic(err)
	}
	a.ewBtnConn = ewBtnConn

	go a.listenButtonEvents(a.nsBtnConn, NS)
	go a.listenButtonEvents(a.ewBtnConn, EW)
	go a.ctl.Run()

	for {
		select {
		case state := <-a.ctl.outbox:
			switch state.(type) {
			case *StateTransitionEvent:
				state := state.(*StateTransitionEvent)
				slog.Info("received state", "state", state)
				a.Apply(a.nsConn, state.To.NS)
				a.Apply(a.ewConn, state.To.EW)
			}
		}
	}
}

func (a *Application) Apply(conn net.Conn, light Light) {
	switch light {
	case Red:
		conn.Write([]byte("R"))
	case Yellow:
		conn.Write([]byte("Y"))
	case Green:
		conn.Write([]byte("G"))
	}
}

func (a *Application) listenButtonEvents(conn *net.UDPConn, ns Target) {
	const bufSize = 1024
	buf := make([]byte, bufSize)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			panic(err)
		}
		if n > 0 {
			slog.Info("received button press", "target", ns)
			a.ctl.inbox <- &ButtonEvent{Target: ns}
		}
	}
}
