package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"
)

var (
	createDuration     = time.Second * 2
	deletedDuration    = time.Second * 2
	serviceTtl         = time.Second * 30
	serviceCntMax      = 1
	connUpperThreshold = 128
	connDownThreshold  = 64
	lastCreated        = time.Time{}
	lastDeleted        = time.Time{}
	connCnt            = make(chan int, 4096)
	services           = []Service{}
	servicePorts       = make(chan int, serviceCntMax)
	config             = make(map[string]interface{})
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
	image := CastToStr(config["docker_image"])
	tag := CastToStr(config["docker_image_tag"])
	host := CastToStr(config["service_host"])
	portOri := CastToInt(config["service_port"])
	cmdFmt := CastToStr(config["docker_command"])
	name := fmt.Sprintf("flask_%v", port)

	cmdStr := fmt.Sprintf(cmdFmt, name, port, portOri, image, tag)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()
	regSucc := registerService(host, port, 10)
	if !regSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}
	service := Service{
		name:    name,
		created: time.Now(),
		host:    host,
		port:    port,
	}
	url := fmt.Sprintf("http://%v:%v", host, port)
	for {
		result := HttpGet(url)
		if result {
			break
		}
	}
	fmt.Println("service created")
	services = append(services, service)
}

func removeServiceInstance(index int) {
	service := services[index]
	defer func() {
		servicePorts <- service.port
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

func newServiceInstanceLazy(port int) {
	image := CastToStr(config["docker_image"])
	tag := CastToStr(config["docker_image_tag"])
	host := CastToStr(config["service_host"])
	portOrigin := CastToInt(config["service_port"])
	name := fmt.Sprintf("%v_%v", image, port)
	pathCheckpointOrigin := CastToStr(config["checkpoint_path"])
	pathCheckpointTemp := CastToStr(config["checkpoint_path_temp"])
	cmdFmt := CastToStr(config["docker_command_criu"])

	pathCheckpoint := fmt.Sprintf("%v/%v", pathCheckpointTemp, port)

	cmdStr := fmt.Sprintf("cp -R --reflink=always -a %v %v", pathCheckpointOrigin, pathCheckpoint)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	cmdStr = fmt.Sprintf(cmdFmt, name, port, portOrigin, image, tag)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	go func() {
		cmdStr = fmt.Sprintf("criu lazy-pages --images-dir %v &", pathCheckpoint)
		_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()
		fmt.Println("lazy-pages done")
	}()

	cmdStr = fmt.Sprintf("docker start --checkpoint-dir=%v --checkpoint=%v %v", pathCheckpointTemp, port, name)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	regSucc := registerService(host, port, 10)
	if !regSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}
	service := Service{
		created: time.Now(),
		name:    name,
		host:    host,
		port:    port,
	}
	url := fmt.Sprintf("http://%v:%v", host, port)
	for {
		result := HttpGet(url)
		if result {
			break
		}
	}
	fmt.Println("service created")
	services = append(services, service)
}

func removeServiceInstanceLazy(index int) {
	service := services[index]
	defer func() {
		servicePorts <- service.port
	}()
	services = append(services[:index], services[index+1:]...)

	cmdStr := fmt.Sprintf("sudo docker stop %v", service.name)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	cmdStr = fmt.Sprintf("sudo docker rm %v", service.name)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	pathCheckpoint := fmt.Sprintf("/volume/flask/temp/%v", service.port)
	cmdStr = fmt.Sprintf("rm -rf %v", pathCheckpoint)
	_, _ = exec.Command("/bin/sh", "-c", cmdStr).Output()

	unregSucc := unregisterService(service.host, service.port)
	if !unregSucc {
		log.Fatal("fail to register watchdog to consul")
		return
	}
	fmt.Println("service removed")
}

func registerService(host string, port int, weight int) bool {
	consulHost := CastToStr(config["consul_host"])
	serviceName := CastToStr(config["consul_service_name"])
	url := fmt.Sprintf("http://%v:8500/v1/kv/upstreams/%v/%v:%v", consulHost, serviceName, host, port)
	data := map[string]int{"weight": weight, "max_fails": 3, "fail_timeout": 10}
	regSucc := HttpPut(url, data)
	if !regSucc {
		log.Fatal("fail to register to consul")
		return false
	}
	return true
}

func unregisterService(host string, port int) bool {
	consulHost := CastToStr(config["consul_host"])
	serviceName := CastToStr(config["consul_service_name"])
	url := fmt.Sprintf("http://%v:8500/v1/kv/upstreams/%v/%v:%v", consulHost, serviceName, host, port)
	regSucc := HttpDelete(url)
	if !regSucc {
		log.Fatal("fail to unregister from consul")
		return false
	}
	return true
}

func schedule() {
	useCriu := CastToBool(config["use_criu"])
	for range time.Tick(time.Millisecond * 100) {
		currentCnt := len(connCnt)
		serviceCnt := len(services)
		concurrency := 0
		if serviceCnt == 0 {
			concurrency = currentCnt
		} else {
			concurrency = currentCnt * (serviceCnt*10 + 1) / serviceCnt
		}
		if currentCnt > 0 && serviceCnt == 0 && len(servicePorts) > 0 {
			if time.Since(lastCreated) > createDuration {
				fmt.Println("new service")
				lastCreated = time.Now()
				if useCriu {
					go newServiceInstanceLazy(<-servicePorts)
				} else {
					go newServiceInstance(<-servicePorts)
				}
			}
		}
		if concurrency > connUpperThreshold && serviceCnt < serviceCntMax && len(servicePorts) > 0 {
			if time.Since(lastCreated) > createDuration {
				fmt.Println("new service")
				lastCreated = time.Now()
				if useCriu {
					go newServiceInstanceLazy(<-servicePorts)
				} else {
					go newServiceInstance(<-servicePorts)
				}
			}
		}
		if concurrency < connDownThreshold {
			if time.Since(lastDeleted) > deletedDuration {
				for index, service := range services {
					if time.Since(service.created) >= serviceTtl {
						fmt.Println("remove service")
						lastDeleted = time.Now()
						if useCriu {
							go removeServiceInstanceLazy(index)
						} else {
							go removeServiceInstance(index)
						}
						break
					}
				}
			}

		}
	}
}

func main() {
	config = GetConfig()

	host := CastToStr(config["host"])
	port := CastToInt(config["port"])
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

	servicePort := CastToInt(config["service_port"])
	serviceCntMax = CastToInt(config["service_cnt_max"])
	servicePorts = make(chan int, serviceCntMax)
	for i := 0; i < serviceCntMax; i++ {
		servicePorts <- servicePort + i
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
