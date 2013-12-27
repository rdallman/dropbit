//Copyright 2013
//TODO Insert Go Modified BSD License here

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
	port   = 6667
)

type share struct {
	Secret string
	Path   string
	Db     *sql.DB
	peers  map[string]net.Addr //map[port:ip]addr
}

type Share interface {
	sendMetaShake(addr *net.UDPAddr)
	receiveMetaShake()
	sendping(addr *net.UDPAddr)
	getping()
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

//bencode doesn't care about capitalization... should we?
type Ping struct {
	M     string   `m`
	Port  int      `port`
	Share [20]byte `share`
	Peer  [20]byte `peer`
}

type Shake struct {
	m       string
	files   []t_file
	port    int
	peer_id [20]byte
	peers   []string
}

type Request struct {
}

type Reply struct {
}

func newShare(secret, path string) share {
	return share{
		secret,
		path,
		nil,
		make(map[string]net.Addr),
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
		shares[secret] = share{Path: s.Path, Db: db, peers: s.peers}
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
			btf, err := getFileInfo(path)
			fmt.Println(path)
			//err here = directory -- we don't need these
			if err != nil {
				return nil
			}
			relPath := path[len(s.Path)+1:]
			var b bytes.Buffer
			err = bencode.Marshal(&b, btf)
			check(err)
			_, err = stmt.Exec(relPath, f.ModTime().String(), b.Bytes())
			check(err)
			return nil
		})
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
			//TODO tricky here...
			s = newShare(secret, s.Path)
			loadShare(secret, s)
		}
	}
}

//uses local absolute path, generates metadata on new file
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

	phash := make(chan int, iters)
	pieces := make([]byte, iters*20)
	for i := 0; i < iters; i++ {
		//TODO redundant channel?
		go func(i int) {
			//FIXME min() not necessary, then it was...
			s := sha1.Sum(d[plength*i : int(math.Min(float64(plength*(i+1)), float64(len(d))))])
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

//listen for pings, hand back metadata (encrypt shake w/ key eventually)
func reedWrite() {
	//TODO get real IP; ifconfig address for now
	me, err := net.ResolveUDPAddr("udp", "192.168.1.64:6667")
	check(err)
	sock, err := net.ListenUDP("udp", me)
	check(err)
	for {
		//this whole size thing is probably bad
		b := make([]byte, 4096)
		_, addr, err := sock.ReadFrom(b)
		check(err)
		if string(b[:4]) == "DBIT" {
			var p Ping
			err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &p)
			check(err)

			for secret, s := range shares {
				if p.Share == sha1.Sum([]byte(secret)) {
					//TODO polling "known hosts" periodically?
					//TODO send broadcast to "known hosts" when change happens? (fsnotify)
					//if s.peers == nil {
					//FIXME potentially solved this new map issue
					//shares[secret] = share{Path: s.Path, Db: s.Db, make(map[string]net.Addr)}
					//s = shares[secret]
					//}
					a := addr.(*net.UDPAddr)
					a.Port = p.Port
					fmt.Println(a.String())
					_, known := s.peers[a.String()]
					if !known {
						s.peers[a.String()] = a
						s.metaShake(me, a)
					}
				}
			}
			//fmt.Println(string(b))
			//var t tracker
			//err = bencode.Unmarshal(bytes.NewBuffer(b), &t)
			//for _, s := range shares {
			//switch t.m {
			//case "meta_shake":
			//s.metaShake(c.(*net.UDPAddr))
			//}
			////sock.WriteTo(b, c)
		}
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

//TODO eh temporary, but may stick w/ new name
type t_file struct {
	path string
	time string
}

func (s *share) sendPing(secret string, target *net.UDPAddr) {
	// s := shares[secret]
	// write(ping)
	// s.listenForMeta()
}

//always be doing this
func (s *share) listenForPing(target *net.UDPAddr) {
	// read(ping)
	// !known? add
	// s := shares[secret]
	// write(s.sendMeta(you))
	// s.listenForMeta()
}

func (s *share) listenForMeta(target *net.UDPAddr) {
	//read(meta)
	// unmarshal(&youMeta)
	// for f in files
	//   hash(you) ==? hash(me)
	//    yours > mine
	//      have = 0
	//      sql.write(f, yourMeta)
	//      request(f)
}

//TODO listen for requests...
// uh session keys and send the secret each time?
// know the secret for each request we get?

func (s *share) metaShake(me, you *net.UDPAddr) {
	conn, err := net.DialUDP("udp", me, you)
	check(err)
	b := createMetaShake(s)
	_, err = conn.Write(b)
	check(err)
	r := make([]byte, 4096)
	_, err = conn.Read(r)
	fmt.Println(string(r))
}

//analagous to getTracker
//TODO do this recursively
//gotta find peers first
//TODO no central tracker?
func createMetaShake(s *share) []byte {
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
	err = bencode.Marshal(&b, Shake{
		"meta_shake",
		files,
		6667,
		sha1.Sum([]byte("192.168.1.64:6667")), //FIXME config?
		peers,
	})
	return b.Bytes()
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
			var r Ping
			err := bencode.Unmarshal(bytes.NewBuffer(b[4:]), &r)
			check(err)

			for secret, s := range shares {
				if r.Share == sha1.Sum([]byte(secret)) {
					//TODO polling "known hosts" periodically?
					//TODO send broadcast to "known hosts" when change happens? (fsnotify)

					//if s.peers == nil {
					////TODO figure out a better way to add db and peers... this is annoying AF
					//shares[secret] = share{s.Path, s.Db, make(map[string]net.Addr)}
					//s = shares[secret]
					//}
					a := addr.(*net.UDPAddr)
					a.Port = r.Port
					fmt.Println(a.String())
					_, known := s.peers[a.String()]
					if !known {
						s.peers[a.String()] = a
						fmt.Println(s.peers)
						//TODO PING
					}
				}
			}
		}
	}
}

func (s *share) Ping(secret string) []byte {
	buf := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(buf, Ping{
		"ping",
		6667,
		sha1.Sum([]byte(secret)),
		//FIXME not sure if Network() is sufficient
		sha1.Sum([]byte("192.168.1.64:6667")),
	})
	check(err)

	return buf.Bytes()
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
		for secret, s := range shares {
			b := s.Ping(secret)

			_, err = sock.Write(b)
			check(err)
		}
		time.Sleep(2 * time.Second)
	}
}

func main() {
	fmt.Printf("%d", time.Now().Unix())
	parseConfig()
	go listenMultiCast()
	go sendMultiCast()
	go reedWrite()
	for {
		time.Sleep(100 * time.Millisecond)
	}
}
