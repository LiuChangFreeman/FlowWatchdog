package main

import (
	"FlowWatchdog/utils"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"
)

var (
	createDuration    = time.Second * 2
	deletedDuration   = time.Second * 2
	servicesMax       = 1
	connUpThreshold   = 128
	connDownThreshold = 64
	lastCreated       = time.Time{}
	lastDeleted       = time.Time{}
	connCnt           = make(chan int, 4096)
	services          = []Service{}
	servicesPorts     = make(chan int, servicesMax)
	config            = make(map[string]interface{})
)

func handleErr(err error) bool {
	if err != nil {
		return true
	} else {
		return false
	}
}

type Service struct {
	name    string
	host    string
	port    int
	created time.Time
}

func handleConn(conn *net.TCPConn) {

	defer conn.Close()
	writeChan := make(chan int, 1)
	var buffTemp [512]byte
	cnt, err := conn.Read(buffTemp[:])
	if handleErr(err) {
		return
	}

	if strings.HasPrefix(string(buffTemp[:cnt]), "GET /check HTTP/1.0") {
		_, err = conn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		if handleErr(err) {
			return
		}
		return
	} else {
		connCnt <- 1
		defer func() {
			<-connCnt
		}()
	}

	for {
		serviceCnt := len(services)
		if serviceCnt > 0 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}

	service := services[rand.Intn(len(services))]
	address := fmt.Sprintf("%v:%v", service.host, service.port)

	serviceConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer serviceConn.Close()

	exit := make(chan bool, 1)

	go func() {
		for {
			n := 0
			for i := 0; i < cnt; i += n {
				n, err = serviceConn.Write(buffTemp[i:cnt])
				if handleErr(err) {
					exit <- true
					return
				}
			}
			writeChan <- 1
			return
		}
	}()

	go func() {
		select {
		case <-writeChan:
			break
		}
		var buff [512]byte
		for {
			m, err := conn.Read(buff[:])
			if handleErr(err) {
				exit <- true
				return
			}

			n := 0
			for i := 0; i < m; i += n {
				n, err = serviceConn.Write(buff[i:m])
				if handleErr(err) {
					exit <- true
					return
				}
			}
		}
	}()

	go func() {
		var buff [512]byte
		for {
			m, err := serviceConn.Read(buff[:])
			if handleErr(err) {
				exit <- true
				return
			}

			n := 0
			for i := 0; i < m; i += n {
				n, err = conn.Write(buff[i:m])
				if handleErr(err) {
					exit <- true
					return
				}
			}
		}
	}()

	select {
	case <-exit:
		return
	}
}

func newServiceInstance(port int) {
	cmdStr := fmt.Sprintf("sudo docker run -id --rm --name flask_%v --security-opt seccomp=unconfined -p 127.0.0.1:%v:9000 -v /home/lazy:/usr/src/dir -w /usr/src/dir flask python /usr/src/dir/lazy.py", port, port)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()
	host := utils.CastToStr(config["service_host"])
	regSucc := registerService(host, port, 10)
	if !regSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}
	name := fmt.Sprintf("flask_%v", port)
	service := Service{
		name:    name,
		created: time.Now(),
		host:    host,
		port:    port,
	}
	url := fmt.Sprintf("http://%v:%v", host, port)
	for {
		result := utils.HttpGet(url)
		if result {
			break
		}
	}
	fmt.Println("service created")
	services = append(services, service)
}

func registerService(host string, port int, weight int) bool {
	consulHost := utils.CastToStr(config["consul_host"])
	serviceName := utils.CastToStr(config["service_name"])
	url := fmt.Sprintf("http://%v:8500/v1/kv/upstreams/%v/%v:%v", consulHost, serviceName, host, port)
	data := map[string]int{"weight": weight, "max_fails": 3, "fail_timeout": 10}
	regSucc := utils.HttpPut(url, data)
	if !regSucc {
		log.Fatal("fail to register to consul")
		return false
	}
	return true
}

func removeServiceInstance(index int) {
	service := services[index]
	defer func() {
		servicesPorts <- service.port
	}()
	services = append(services[:index], services[index+1:]...)
	cmdStr := fmt.Sprintf("sudo docker stop %v", service.name)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()
	unregSucc := unregisterService(service.host, service.port)
	if !unregSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}
	fmt.Println("service removed")
}

func unregisterService(host string, port int) bool {
	consulHost := utils.CastToStr(config["consul_host"])
	serviceName := utils.CastToStr(config["service_name"])
	url := fmt.Sprintf("http://%v:8500/v1/kv/upstreams/%v/%v:%v", consulHost, serviceName, host, port)
	regSucc := utils.HttpDelete(url)
	if !regSucc {
		log.Fatal("fail to unregister from consul")
		return false
	}
	return true
}

func schedule() {
	for range time.Tick(time.Millisecond * 100) {
		currentCnt := len(connCnt)
		serviceCnt := len(services)
		concurrency := 0
		if serviceCnt == 0 {
			concurrency = currentCnt
		} else {
			concurrency = currentCnt * (serviceCnt*10 + 1) / serviceCnt
		}
		if currentCnt > 0 && serviceCnt == 0 {
			if time.Since(lastCreated) > createDuration {
				fmt.Println("new service")
				lastCreated = time.Now()
				go newServiceInstance(<-servicesPorts)
			}
		}
		if concurrency > connUpThreshold && serviceCnt < servicesMax {
			if time.Since(lastCreated) > createDuration {
				fmt.Println("new service")
				lastCreated = time.Now()
				go newServiceInstance(<-servicesPorts)
			}
		}
		if concurrency < connDownThreshold {
			if time.Since(lastDeleted) > deletedDuration {
				for index, service := range services {
					if time.Since(service.created) >= time.Second*10 {
						fmt.Println("remove service")
						lastDeleted = time.Now()
						go removeServiceInstance(index)
						break
					}
				}
			}

		}
	}
}

func main() {
	config = utils.GetConfig()

	host := utils.CastToStr(config["host"])
	port := utils.CastToInt(config["port"])
	address := net.TCPAddr{
		IP:   net.ParseIP(host),
		Port: port,
	}

	listener, err := net.ListenTCP("tcp", &address)
	if err != nil {
		log.Fatal("fail to listen to ", address, err)
	}

	regSucc := registerService(host, port, 1)
	if !regSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}

	servicePort := utils.CastToInt(config["service_port"])
	for i := 0; i < servicesMax; i++ {
		servicesPorts <- servicePort + i
	}

	defer func() {
		unregisterService(host, port)
	}()

	go schedule()

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
