package main

import (
	"fmt"
	"net"
)

func main() {
	client, err := net.Dial("tcp", "python.org:80")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	request := "GET / HTTP/1.1\r\n" +
		"Host: go.dev\r\n" +
		"\r\n"

	_, err = client.Write([]byte(request))
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 4096)
	for {
		n, err := client.Read(buf)
		if err != nil {
			panic(err)
		}
		if n == 0 {
			break
		}
		fmt.Print(string(buf[:n]))
	}
}
