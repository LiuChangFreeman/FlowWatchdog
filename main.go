package main

import (
	"log"
	"net"
)

var (
	connCount = make(chan int, 4096)
)

func handleErr(err error) bool {
	if err != nil {
		return true
	} else {
		return false
	}
}

func handleConn(conn *net.TCPConn) {
	serviceConn, err := net.Dial("tcp", "172.24.9.246:8080")
	if err != nil {
		log.Fatal(err)
		return
	}

	connCount <- 1
	defer serviceConn.Close()
	defer conn.Close()
	defer func() {
		<-connCount
	}()

	done := make(chan bool)

	go func() {
		var buff [512]byte
		for {
			n, err := conn.Read(buff[0:])
			if handleErr(err) {
				done <- true
				return
			}

			n, err = serviceConn.Write(buff[0:n])
			if handleErr(err) {
				done <- true
				return
			}
		}
	}()

	go func() {
		var buff [512]byte
		for {
			n, err := serviceConn.Read(buff[0:])
			if handleErr(err) {
				done <- true
				return
			}

			n, err = conn.Write(buff[0:n])
			if handleErr(err) {
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		return
	}
}

func main() {

	address := net.TCPAddr{
		IP:   net.ParseIP("192.168.31.125"),
		Port: 8000,
	}

	listener, err := net.ListenTCP("tcp", &address)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
