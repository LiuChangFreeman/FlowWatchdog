package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	service := "127.0.0.1:8000"
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		n, err := conn.Write([]byte("HEAD / HTTP/1.1\r\n\r\n"))
		if err != nil {
			log.Fatal(err)
		}

		var buff [512]byte
		n, err = conn.Read(buff[0:])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(buff[0:n]))
		time.Sleep(time.Second * 5)
	}
}
