package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"encoding/json"
	"io/ioutil"
	"strconv"
	//"os"
	//"path/filepath"
	//"encoding/base32"
	"fmt"
	"net"
	"time"
)

func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

var secrets []string

func parseConfig() {
	f, err := ioutil.ReadFile("./test.conf")
	check(err)
	shares := []struct {
		Secret string
		Path   string
	}{}
	json.Unmarshal(f, &shares)
	for _, s := range shares {
		secrets = append(secrets, s.Secret)
	}
}

func reedWrite() {
	//TODO guess locally i recognize myself different? should be non-issue
	addr, err := net.ResolveUDPAddr("udp", "192.168.1.64:6667")
	check(err)
	sock, err := net.ListenUDP("udp", addr)
	check(err)
	for {
		b := make([]byte, 4096)
		_, c, err := sock.ReadFrom(b)
		check(err)
		fmt.Println(string(b))
		sock.WriteTo(b, c)
	}
}

func writeReed(addr *net.UDPAddr, port int) {
	a, err := net.ResolveUDPAddr("udp", addr.IP.String()+":"+strconv.Itoa(port))
	c, err := net.DialUDP("udp", nil, a)
	check(err)
	b := []byte("hello")
	_, err = c.Write(b)
	check(err)
	_, err = c.Read(b)
	check(err)
}

//TODO maybe won't need this, but just in case
func getMulticastAddrs() []net.Addr {
	var addr []net.Addr
	is, _ := net.Interfaces()
	for _, i := range is {
		addrs, _ := i.MulticastAddrs()
		for _, a := range addrs {
			addr = append(addr, a)
			fmt.Println(a)
		}
	}
	return addr
}

func listenMultiCast() {
	//TODO use getMulticastAddrs() to do this dynamically? necessary?
	addr, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	sock, err := net.ListenMulticastUDP("udp", nil, addr)
	check(err)
	for {
		b := make([]byte, 4096)
		_, addr, err := sock.ReadFrom(b)
		check(err)
		if string(b[:4]) == "DBIT" {
			var r BCast
			err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &r)
			check(err)

			for _, s := range secrets {
				if r.Share == sha1.Sum([]byte(s)) {
					//spawn socket
					//TODO make "known hosts"
					//  if unknown
					//    add
					//  else
					//    nothing
					//
					//TODO polling "known hosts" periodically?
					//TODO send broadcast to "known hosts" when change happens?
					writeReed(addr.(*net.UDPAddr), r.Port)
				}
			}
			//h := sha1.New()
			//filepath.Walk("./", func(path string, f os.FileInfo, err error) error {
			//io.WriteString(h, f.ModTime().String())
			//return nil
			//})

			//fhash := h.Sum(nil)
			//_ = fhash
		}
	}
}

type BCast struct {
	M     string   `m`
	Port  int      `port`
	Share [20]byte `share`
	Peer  string   `peer`
}

//key:
//AZYZ6M7P34KK7W5RQB6AAOEQEG2XT2VLG

//BSYNCd1:m4:ping4:peer20:.Ro;F-4:porti60021e5:share20::i}\#g#z4qe

//TODO
//for s in secrets
//  for a in addrs
func sendMultiCast() {
	addr, err := net.ResolveUDPAddr("udp", "239.192.0.0:3838")
	sock, err := net.DialUDP("udp", nil, addr)
	check(err)
	for {
		buf := bytes.NewBuffer([]byte("DBIT"))
		err := bencode.Marshal(buf, BCast{
			"ping",
			6667,
			sha1.Sum([]byte(secrets[0])),
			"poop",
		})

		check(err)
		_, err = sock.Write(buf.Bytes())
		check(err)
		time.Sleep(2 * time.Second)
	}
}

func main() {
	go reedWrite()
	go listenMultiCast()
	go sendMultiCast()
	parseConfig()
	for {
		time.Sleep(100 * time.Millisecond)
	}
}
