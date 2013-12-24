//Copyright 2013
//TODO Insert Go Modified BSD here

//TODO this is messy as fuck, I know
//step #1: make it work
//step #2: make it fast and pretty
//step #3: go outside
//
//current step:
// [1] 2 3

package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	//"encoding/base32"
	"fmt"
	"net"
	"time"
)

var (
	shares = make(map[string]share) //map[secret]share
)

type share struct {
	Path  string
	Db    *sql.DB
	peers map[string]net.Addr //map[port:ip]addr
}

type Share interface {
	metaShake(addr *net.UDPAddr)
}

const (
	BLOCK_SIZE   = 1 << 14 //2^14
	PIECE_LENGTH = 1 << 18 //2^18
)

func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func loadShare(secret string, s share) {
	newShare := false
	//if file doesn't exist, need to drop some tables
	//has to be done here because sql.Open will make a file
	if _, err := os.Stat(secret + ".db"); os.IsNotExist(err) {
		fmt.Println("new share")
		newShare = true
	}
	//hopefully reload config will not reload the database
	if shares[secret].Db == nil {
		fmt.Println("no db")
		db, err := sql.Open("sqlite3", secret+".db")
		check(err)
		shares[secret] = share{s.Path, db, s.peers}
	}
	//drop some tables
	if newShare {
		//TODO potential bottleneck here, walk is slow -- but must be done somehow
		//  it appears there are about 20 ways to do io in stdlib
		//TODO time not in sync db, they parse bencoding -- maybe a good idea?
		//      allows: select * from files where time > x;  x = most recent, gets all new
		db := shares[secret].Db
		_, err := db.Exec(
			`CREATE TABLE files (
          path TEXT NOT NULL PRIMARY KEY,
          time TEXT NOT NULL,
          data BLOB NOT NULL);`)
		check(err)
		//} // end newShare here
		stmt, err := db.Prepare("insert into files(path, time, data) values(?,?,?)")

		check(err)

		filepath.Walk(s.Path, func(path string, f os.FileInfo, err error) error {
			fmt.Println(path)
			btf, err := getFileInfo(path)
			fmt.Println(path)
			//err mostly for directories
			//slice off sync abs path + /
			//relPath := path[len(s.Path)+1:]
			relPath := path
			var b bytes.Buffer
			err = bencode.Marshal(&b, btf)
			check(err)
			_, err = stmt.Exec(relPath, f.ModTime().String(), b.Bytes())
			check(err)
			return nil
		})
		fmt.Println("never got here")
	}
}

//TODO watch changes on this file to reload it
//TODO put these in home ~/.dbit/config for linux
//for initial load / reloading config file
func parseConfig() {
	f, err := ioutil.ReadFile("./test.conf")
	if shares == nil {
		shares = make(map[string]share)
	}
	cshares := make(map[string]share)
	err = json.Unmarshal(f, &cshares)
	check(err)
	//TODO check equality of some traits to see if we need a reload
	// ...once the config actually has some settings
	for secret, s := range cshares {
		_, ok := shares[secret]
		if !ok {
			fmt.Println("added")
			loadShare(secret, s)
		}
	}
}

func getFileInfo(path string) (bt bt_file, err error) {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return bt, err
	}

	//TODO compute this smarter, not just min(256k, len(file))
	plength := int(math.Min(float64(PIECE_LENGTH), float64(len(d))))

	if plength == 0 {
		return bt, err
	}
	iters := len(d) / plength
	if len(d)%plength > 0 {
		iters += 1
	}
	//on the off chance of perfection...

	phash := make(chan int, iters)
	pieces := make([]byte, iters*20)
	for i := 0; i < iters; i++ {
		//TODO need concurrency bad... maybe not, redundant channel?
		go func(i int) {
			//FIXME min() not necessary, then it was...
			s := sha1.Sum(d[plength*i : int(math.Min(float64(plength*(i+1)), float64(len(d))))])
			//TODO make sure this actually works...
			pieces = append(pieces[:(i)*20], append(s[:], pieces[(i)*20:]...)...)
			phash <- 1
		}(i)
	}
	<-phash
	return bt_file{int64(len(d)), plength, string(pieces)}, nil
}

//SPEC theres a map[filename]these floating around
type bt_file struct {
	length       int64
	piece_length int
	pieces       string
}

func reedWrite() {
	//TODO guess locally I recognize myself different? should be non-issue
	addr, err := net.ResolveUDPAddr("udp", "192.168.1.64:6667")
	check(err)
	sock, err := net.ListenUDP("udp", addr)
	check(err)
	for {
		b := make([]byte, 4096)
		_, _, err := sock.ReadFrom(b)
		check(err)
		fmt.Println(string(b))
		//var r bt_req
		//err = bencode.Unmarshal(bytes.NewBuffer(b), &r)
		//switch r.req_type {
		//case 0:
		//reply(sock, c, r.piece)
		//case 1:
		//}
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
		//TODO truncate []byte?
	}
	//go func() {
	_, err = c.WriteTo(b, addr)
	check(err)
	//ch <- b
	//}()

	//<-ch
}

type tracker struct {
	m       string
	files   []t_file
	port    int
	ip      [4]byte
	peer_id [20]byte
	peers   []string
}

//TODO eh temporary, but may stick w/ new name
type t_file struct {
	path string
	time string
}

//analagous to getTracker
//TODO do this recursively
//gotta find peers first
//TODO no central tracker?
func (s *share) metaShake(addr *net.UDPAddr) {
	c, err := net.DialUDP("udp", nil, addr)
	check(err)
	fmt.Println("shaking")

	//TODO see myself doing this a lot... extract func
	rows, err := s.Db.Query("SELECT path, time FROM files")
	check(err)
	files := make([]t_file, 0)
	for rows.Next() {
		var path, time string
		rows.Scan(&path, &time)
		files = append(files, t_file{path, time})
	}
	rows.Close()

	peers := make([]string, 0)
	for p, _ := range s.peers {
		peers = append(peers, p)
	}

	var b bytes.Buffer
	err = bencode.Marshal(&b, tracker{
		"meta_shake",
		files,
		6667,
		[4]byte{192, 168, 1, 64},
		sha1.Sum([]byte("192.168.1.64:6667")), //FIXME config?
		peers,
	})
	check(err)
	_, err = c.Write(b.Bytes())
	check(err)
	r := make([]byte, 4096)
	_, err = c.Read(r)
	fmt.Println(string(r))
}

func writeReed(addr *net.UDPAddr, port int) {
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
		b := make([]byte, 256)
		_, addr, err := sock.ReadFrom(b)
		check(err)
		if string(b[:4]) == "DBIT" {
			var r BCast
			err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &r)
			check(err)

			for secret, s := range shares {
				if r.Share == sha1.Sum([]byte(secret)) {
					//TODO polling "known hosts" periodically?
					//TODO send broadcast to "known hosts" when change happens? (fsnotify)
					if s.peers == nil {
						s.peers = make(map[string]net.Addr)
					}
					_, known := s.peers[addr.Network()]
					if !known {
						a := addr.(*net.UDPAddr)
						a.Port = r.Port
						s.peers[a.Network()] = a
						s.metaShake(a)
					}
				}
			}
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
		for s, _ := range shares {
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
	parseConfig()
	parseConfig()
	go listenMultiCast()
	go sendMultiCast()
	for {
		time.Sleep(100 * time.Millisecond)
	}
}
