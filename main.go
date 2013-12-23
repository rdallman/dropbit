package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	//"encoding/base32"
	"fmt"
	"net"
	"time"
)

//secret : path
var folders = make(map[string]string)

const BLOCK_SIZE = 16 * 1024

func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

//TODO watch changes on this file to reload it
//TODO put these in home ~/.config/dbit/ for linux
func parseConfig() {
	f, err := ioutil.ReadFile("./test.conf")
	check(err)
	shares := []struct {
		Secret string
		Path   string
	}{}
	json.Unmarshal(f, &shares)
	//map secrets to absolute paths in mem to know what to do when we get one
	for _, s := range shares {
		folders[s.Secret] = s.Path
		_, err := os.Open("./" + s.Secret)
		//if this is a new share
		if err != nil {
			f, err := os.Create("./" + s.Secret + ".torrent")
			check(err)

			var buf bytes.Buffer
			bt_files := make([]bt_file, 0)
			//TODO append metadata
			//TODO potential bottleneck here, walk is slow -- but must be done somehow
			//  it appears there are about 20 ways to do io in stdlib
			filepath.Walk(s.Path, func(path string, f os.FileInfo, err error) error {
				d, err := ioutil.ReadFile(path)
				buf.Write(d)
				bt_files = append(bt_files, bt_file{f.Size(), path})
				return nil
			})

			pieces := buf.Bytes()
			var plength int
			if len(pieces) < BLOCK_SIZE {
				plength = len(pieces)
			} else {
				plength = BLOCK_SIZE
			}

			iters := len(pieces) / plength
			//on the off chance of perfection...
			if len(pieces)%plength > 0 {
				iters += 1
			}

			phash := make(chan int, iters)
			realp := make([]byte, iters*20)
			for i := 0; i < iters; i++ {
				//TODO need concurrency bad
				go func(i int) {
					s := sha1.Sum(pieces[plength*i : (i+1)*plength+1])
					//TODO make sure this actually works...
					realp = append(realp[:(i)*20], append(s[:], realp[(i)*20:]...)...)
					//fmt.Println(pieces[plength*i : (i+1)*plength+1])
					//fmt.Println(s[:])
					_ = s
					phash <- 1
				}(i)
			}
			<-phash

			infos := struct {
				piece_length int
				pieces       string
				files        []bt_file
			}{
				plength,
				string(realp),
				bt_files,
			}
			fmt.Println(len(realp))

			var b bytes.Buffer
			bencode.Marshal(&b, infos)
			f.Write(b.Bytes())
			f.Close()
		}
	}
}

type bt_file struct {
	length int64
	path   string
}

func reedWrite() {
	//TODO guess locally if recognize myself different? should be non-issue
	addr, err := net.ResolveUDPAddr("udp", "192.168.1.64:6667")
	check(err)
	sock, err := net.ListenUDP("udp", addr)
	check(err)
	for {
		b := make([]byte, 4096)
		_, c, err := sock.ReadFrom(b)
		check(err)
		fmt.Println(string(b))
		var r bt_req
		err = bencode.Unmarshal(bytes.NewBuffer(b), &r)
		switch r.req_type {
		case 0:
			reply(sock, c, r.piece)
		case 1:
		}
		//sock.WriteTo(b, c)
	}
}

func reply(c *net.UDPConn, addr net.Addr, part int64) {
	f, err := os.Open("shit")
	check(err)
	//fi, err := f.Stat()
	//check(err)
	//var i int64 = 0
	//numChans := fi.Size() / 16
	//ch := make(chan []byte, numChans)
	//for ; i*16 < fi.Size(); i++ {
	b := make([]byte, 16)
	n, err := f.ReadAt(b, int64(part*16))
	if n < len(b) && err == io.EOF {
		//truncate []byte?
	}
	//go func() {
	_, err = c.WriteTo(b, addr)
	check(err)
	//ch <- b
	//}()

	//<-ch
}

type bt_req struct {
	req_type int
	filename string
	piece    int64
}

func request(c *net.UDPConn, part int64) {
	var b bytes.Buffer
	err := bencode.Marshal(&b, bt_req{
		0,
		"shit",
		part,
	})
	check(err)
	_, err = c.Write(b.Bytes())
	check(err)
	r := make([]byte, 4096)
	_, err = c.Read(r)
	fmt.Println(string(r))
}

func writeReed(addr *net.UDPAddr, port int) {
	a, err := net.ResolveUDPAddr("udp", addr.IP.String()+":"+strconv.Itoa(port))
	c, err := net.DialUDP("udp", nil, a)
	check(err)
	request(c, 0)

	//TODO reduce below to only shoot needed chunks
	//receiver sends needed chunk, reply with only that
	//TODO figure out if reading or writing
	//write out all chunks of a file

}

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

			for s, _ := range folders {
				if r.Share == sha1.Sum([]byte(s)) {
					//spawn socket
					//TODO make "known hosts"
					//  if unknown
					//    add
					//  else
					//    nothing
					//
					//TODO polling "known hosts" periodically?
					//TODO send broadcast to "known hosts" when change happens? (fsnotify)
					writeReed(addr.(*net.UDPAddr), r.Port)
				}
			}
			////this computes hash of most recent change in directory, recursively
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
	Peer  [20]byte `peer`
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
		//broadcast each of our syncs
		for s, _ := range folders {
			buf := bytes.NewBuffer([]byte("DBIT"))
			err := bencode.Marshal(buf, BCast{
				"ping",
				6667,
				sha1.Sum([]byte(s)),
				//FIXME not sure if Network() is sufficient
				sha1.Sum([]byte(addr.Network())),
			})

			check(err)
			_, err = sock.Write(buf.Bytes())
			check(err)
		}
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
