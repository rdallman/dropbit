package main

import (
	"fmt"
	"net"
	"time"
)

//TODO definitely use this
func getMulticastAddrs() []net.Addr {
	var addr []net.Addr
	is, _ := net.Interfaces()
	for _, i := range is {
		addrs, _ := i.MulticastAddrs()
		for _, a := range addrs {
			//TODO only use IPv4?
			addr = append(addr, a)
			fmt.Println(a)
		}
	}
	return addr
}

func sendMultiCast() {
	addr, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	sock, err := net.DialUDP("udp", nil, addr)
	check(err)
	for {
		//broadcast each of our syncs
		for secret, s := range shares {
			b := s.createPing(secret)

			_, err = sock.Write(b)
			check(err)
		}
		time.Sleep(2 * time.Second)
	}
}
