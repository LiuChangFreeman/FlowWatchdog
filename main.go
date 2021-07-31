package main

import (
	"io"
	"log"
	"net"
)

var (
	count = make(chan int, 1024)
)

func handleErr(err error) bool {
	if err != nil {
		if err != io.EOF {
			return true
		} else {
			return true
		}
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

	count <- 1
	defer serviceConn.Close()
	defer conn.Close()
	defer func() {
		<-count
	}()

	done := make(chan bool)

	go func() {
		var buff [512]byte
		for {
			n, err := conn.Read(buff[0:])
			if handleErr(err) || n == 0 {
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
			if handleErr(err) || n == 0 {
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
