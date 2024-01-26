package main

import (
	"net"
)

func main() {
	conf := Config{
		NSServerAddr: "localhost:8080",
		EWServerAddr: "localhost:8081",
		NSButtonAddr: &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 8082,
		},
		EWButtonAddr: &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 8083,
		}}

	app := NewApplication(conf)
	app.Run()
}
