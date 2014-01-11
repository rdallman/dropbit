package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"fmt"
	"net"
	"strconv"
)

type UDPMessage struct {
	addr *net.UDPAddr
	data []byte
}

//map[address]conn
var peers = make(map[string]*net.UDPConn)

func listen(msg chan UDPMessage) {
	me, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(*port))
	mcast, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	lan, err := net.ListenMulticastUDP("udp", nil, mcast)
	sock, err := net.ListenUDP("udp", me)
	sock.SetReadBuffer(1 << 16) //64K
	fmt.Println(me.String())
	fmt.Println(sock.LocalAddr(), sock.RemoteAddr())
	check(err)

	l := func(c *net.UDPConn) {
		for {
			b := make([]byte, 65000)
			_, addr, err := c.ReadFromUDP(b)
			if err != nil {
				continue
			}
			msg <- UDPMessage{addr, b}
		}
	}
	go l(lan)
	go l(sock)
}

//hash is the hashed secret sent over the wire
func getShare(hash string) (share, error) {
	for secret, s := range shares {
		fmt.Printf("%s\n", hash)
		fmt.Printf("%s\n", sha1.Sum([]byte(secret)))
		if hash == fmt.Sprintf("%s", sha1.Sum([]byte(secret))) {
			return s, nil
		}
	}
	return share{}, fmt.Errorf("Not a common share")
}

//p == nil if not a ping
func parseHeader(b []byte) (Header, error) {
	var h Header
	if string(b[:4]) == "DBIT" {
		err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &h)
		if err != nil {
			return h, fmt.Errorf("Invalid Bencoding")
		}
		return h, nil
	}
	return h, fmt.Errorf("Not a Dropbit message")
}

func handleMessage(m UDPMessage, out chan UDPMessage) {
	addr, b := m.addr, m.data

	h, err := parseHeader(b)
	if err != nil {
		return
	}
	s, err := getShare(h.Share)
	if err != nil {
		return
	}
	//above pretty much figures out for me or not
	//below sees if new peer
	//TODO consider seperate channel for peer discovery
	//TODO also this can be done elsewhere
	addr = changePort(addr, h.Port)
	fmt.Println(addr)
	_, known := s.peers[addr.String()]
	if !known {
		s.peers[addr.String()] = addr
		out <- UDPMessage{addr, s.createMetaShake()}
	}
	b = b[4:] //slice off DBIT

	switch h.M {
	case "ping":
		fmt.Println("do ping")
	case "meta":
		//TODO eh this just feels wrong, unmute this shit
		s.processMeta(UDPMessage{addr, b}, out)
	case "req":
		m, err := s.processRequest(b)
		if err != nil {
			return
		}
		out <- UDPMessage{addr, m}
	case "have":
	case "piece":
		s.processPiece(UDPMessage{addr, b}, out)
	}
}

func sendMessage(m *UDPMessage) {
	conn, ok := peers[m.addr.String()]
	var err error
	if !ok {
		conn, err = net.DialUDP("udp", nil, m.addr)
		check(err)
		if err != nil {
			return
		}
		peers[m.addr.String()] = conn
	}
	conn.SetWriteBuffer(len(m.data))
	_, err = conn.Write(m.data)
	check(err)
}

func changePort(addr *net.UDPAddr, port int) *net.UDPAddr {
	addr.Port = port
	return addr
}
