package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func GetConfig() map[string]interface{} {
	pwd, _ := os.Getwd()
	path := fmt.Sprintf("%v/conf/config.json", pwd)
	buff, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicln("load config conf failed: ", err)
	}
	config := make(map[string]interface{})
	err = json.Unmarshal(buff, &config)
	if err != nil {
		log.Panicln("decode config file failed:", string(buff), err)
	}
	return config
}

func CastToStr(t interface{}) string {
	return t.(string)
}

func CastToInt(t interface{}) int {
	return int(t.(float64))
}

func CastToBool(t interface{}) bool {
	return t.(bool)
}

func HttpPut(url string, data map[string]int) bool {
	client := &http.Client{}
	dataJson, err := json.Marshal(data)
	body := string(dataJson)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest("PUT", url, strings.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	var buff [128]byte
	n, err := res.Body.Read(buff[:])
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	result := string(buff[:n])
	if result == "true" {
		return true
	}
	return false
}

func HttpGet(url string) bool {
	client := &http.Client{
		Timeout: time.Millisecond * 100,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	var buff [128]byte
	_, err = res.Body.Read(buff[:])
	if err != nil && err != io.EOF {
		return false
	}
	return true
}

func HttpDelete(url string) bool {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.ContentLength = 0
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	var buff [64]byte
	n, err := res.Body.Read(buff[:])
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	result := string(buff[:n])
	if result == "true" {
		return true
	}
	return false
}
