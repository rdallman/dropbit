package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"fmt"
	"net"
)

func listen(msg chan UDPMessage) {
	me, err := net.ResolveUDPAddr("udp", ":6667")
	mcast, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	lan, err := net.ListenMulticastUDP("udp", nil, mcast)
	sock, err := net.ListenUDP("udp", me)
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

type DBIT struct {
	M     string `m`
	Port  int    `port`
	Share string `share`
	Peer  string `peer`
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
func parseHeader(b []byte) (DBIT, error) {
	var h DBIT
	if string(b[:4]) == "DBIT" {
		err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &h)
		if err != nil {
			return h, fmt.Errorf("Invalid Bencoding")
		}
		return h, nil
	}
	return h, fmt.Errorf("Not a Dropbit message")
}

type UDPMessage struct {
	addr *net.UDPAddr
	data []byte
}

func handleMessage(addr *net.UDPAddr, b []byte) {
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
	addr = changePort(addr, h.Port)
	_, known := s.peers[addr.String()]
	if !known {
		s.peers[addr.String()] = addr
		s.sendMeta(addr)
	}
	b = b[4:] //slice off DBIT

	switch h.M {
	case "ping":
		fmt.Println("do ping")
	case "meta":
		s.processMeta(b, addr)
	case "req":
		s.processRequest(b, addr)
	case "have":
	case "piece":
		s.processPiece(b, addr)
	}
}

//map[path]hash(torrent)
//eh maybe err
func (s *share) getMyFiles() map[string]string {
	rows, err := s.Db.Query("SELECT path, data FROM files")
	check(err)
	files := make(map[string]string)
	for rows.Next() {
		var path string
		var data []byte
		rows.Scan(&path, &data)
		files[path] = fmt.Sprintf("%s", sha1.Sum(data))
	}
	rows.Close()
	return files
}

func (s *share) processMeta(msg []byte, sender *net.UDPAddr) {
	fmt.Println("process meta")
	mfiles := s.getMyFiles()

	var shake Shake
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &shake)
	check(err)

	yfiles := shake.Files

	for yf, yhash := range yfiles {
		mhash, ok := mfiles[yf]
		fmt.Println(mhash)
		fmt.Println(yhash)
		if !ok || mhash != yhash {
			s.requestFile(yf, sender)
			fmt.Println("requesting ", yf)
		}
	}
}

func (s *share) processRequest(msg []byte, sender *net.UDPAddr) {
	fmt.Println("process request")
	var r Request
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &r)
	check(err)
	fmt.Println(r)
	if r.Index == -1 && r.Begin == -1 && r.Length == -1 {
		//var data bytes.Buffer
		//err := s.Db.QueryRow(
		//"SELECT data",
		//"FROM files",
		//"WHERE path = ?", r.File).Scan(data)
		//check(err)
		var data []byte
		err := s.Db.QueryRow("SELECT data FROM files WHERE path = ?", r.File).Scan(&data)
		check(err)
		fmt.Printf("request for %s\n", r.File)
		p := s.createPiece(r.File, -1, -1, data)
		s.send(p, sender)
	}
	check(err)
}

func (s *share) createPiece(path string, index, begin int64, piece []byte) []byte {
	b := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(b, Piece{
		"piece",
		6667,
		fmt.Sprintf("%s", sha1.Sum([]byte(s.Secret))),
		fmt.Sprintf("%s", sha1.Sum([]byte("192.168.1.64:6667"))),
		path,
		index,
		begin,
		piece,
	})
	check(err)
	return b.Bytes()
}

func (s *share) processPiece(msg []byte, sender *net.UDPAddr) {
	fmt.Println("Process piece")
	var p Piece
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &p)
	check(err)

	fmt.Printf("opening %s to write %s at %d, %d", p.File, string(p.Piece), p.Index, p.Begin)
}

func (s *share) send(body []byte, target *net.UDPAddr) {
	conn, err := net.DialUDP("udp", nil, target)
	_, err = conn.Write(body)
	check(err)
}

func (s *share) requestFile(path string, target *net.UDPAddr) {
	req := s.createRequest(path, -1, -1, -1)
	s.send(req, target)
}

func (s *share) createRequest(path string, index, begin, length int64) []byte {
	b := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(b, Request{
		"req",
		6667,
		fmt.Sprintf("%s", sha1.Sum([]byte(s.Secret))),
		fmt.Sprintf("%s", sha1.Sum([]byte("192.168.1.64:6667"))),
		path,
		index,
		begin,
		length,
	})
	check(err)
	return b.Bytes()
}

func changePort(addr *net.UDPAddr, port int) *net.UDPAddr {
	addr.Port = port
	return addr
}

func newmain() {
	//discover motha fuckas
	parseConfig()
	conChan := make(chan UDPMessage)
	listen(conChan)

	for {
		select {
		case c := <-conChan:
			go handleMessage(c.addr, c.data)
		}
	}
}
