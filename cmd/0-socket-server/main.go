package main

import (
	"log/slog"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("Error accepting connection: %s", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	l, err := conn.Read(buf)
	if err != nil {
		slog.Error("Error reading: %s", err)
		return
	}
	slog.Info("request", "msg", string(buf[:l]))
}
