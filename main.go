package main

import (
	"fmt"
	"net"
	"time"
)

func reedWrite() {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:6667")
	if err != nil {
		fmt.Println(err)
	}
	sock, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println(err)
	}
	for {
		var b []byte
		_, c, err := sock.ReadFrom(b)
		if err != nil {
			fmt.Println(err)
		}
		sock.WriteTo(b, c)
	}
}

func writeReed() {
	addr, err := net.ResolveUDPAddr("udp", "localhost:6667")
	if err != nil {
		fmt.Println(err)
	}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(err)
	}
	b := []byte("hello")
	_, err = c.Write(b)
	if err != nil {
		fmt.Println(err)
	}
	_, _, err = c.ReadFrom(b)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}

func listenMultiCast() {
	addr, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	if err != nil {
		fmt.Println(err)
	}
	sock, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(err)
	}
	for {
		b := make([]byte, 4096)
		_, _, err := sock.ReadFrom(b)
		if err != nil {
			fmt.Println(err)
		}
		if string(b[:5]) == "hello" {
			fmt.Println(string(b))
		}
	}
}

func sendMultiCast() {
	addr, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	if err != nil {
		fmt.Println(err)
	}
	sock, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(err)
	}
	if err != nil {
		fmt.Println(err)
	}
	for {
		if err != nil {
			fmt.Println(err)
		}
		b := []byte("hello")
		_, err = sock.Write(b)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("send hello")
		time.Sleep(2 * time.Second)
	}
}

func main() {
	go reedWrite()
	go listenMultiCast()
	go sendMultiCast()
	for {
		time.Sleep(100 * time.Millisecond)
	}
}
