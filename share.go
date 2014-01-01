package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net"
)

type share struct {
	Secret string
	Path   string
	Db     *sql.DB
	peers  map[string]net.Addr //map[port:ip]addr
}

//TODO this could be useful for maps and DB
func newShare(secret, path string) share {
	return share{
		secret,
		path,
		nil,
		make(map[string]net.Addr),
	}
}

//return map[path]hash(torrent)
//eh maybe err, maybe not the hash, do that later
func (s *share) getFileHashes() map[string]string {
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

func (s *share) processMeta(msg []byte, sender *net.UDPAddr, out chan UDPMessage) {
	fmt.Println("process meta")
	mfiles := s.getFileHashes()

	var shake Shake
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &shake)
	check(err)

	yfiles := shake.Files

	for yf, yhash := range yfiles {
		mhash, ok := mfiles[yf]
		fmt.Println(mhash)
		fmt.Println(yhash)
		if !ok || mhash != yhash {
			b := s.createRequest(yf, -1, -1, -1)
			out <- UDPMessage{sender, b}
		}
	}
}

func (s *share) processRequest(msg []byte) []byte {
	fmt.Println("process request")
	var r Request
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &r)
	check(err)
	fmt.Println(r)
	if r.Index == -1 && r.Begin == -1 && r.Length == -1 {
		var data []byte
		err := s.Db.QueryRow("SELECT data FROM files WHERE path = ?", r.File).Scan(&data)
		check(err)
		fmt.Printf("request for %s\n", r.File)
		return s.createPiece(r.File, -1, -1, data)
	}
	check(err)
	return []byte{}
}

func (s *share) processPiece(msg []byte, out chan UDPMessage) {
	fmt.Println("Process piece")
	var p Piece
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &p)
	check(err)

	if p.Index == -1 && p.Begin == -1 {
		fmt.Println("got meta")
	}
	fmt.Printf("opening %s to write %s at %d, %d\n", p.File, string(p.Piece), p.Index, p.Begin)
}

func (s *share) createPing(secret string) []byte {
	buf := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(buf, Header{
		"ping",
		6667,
		fmt.Sprintf("%s", sha1.Sum([]byte(secret))),
		fmt.Sprintf("%s", sha1.Sum([]byte("192.168.1.64:6667"))),
	})
	check(err)

	return buf.Bytes()
}

//analagous to getTracker
//TODO do this recursively
//gotta find peers first
func (s *share) createMetaShake() []byte {
	files := s.getFileHashes()

	peers := make([]string, 0)
	for p, _ := range s.peers {
		peers = append(peers, p)
	}

	b := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(b, Shake{
		"meta",
		6667,
		fmt.Sprintf("%s", sha1.Sum([]byte(s.Secret))),
		fmt.Sprintf("%s", sha1.Sum([]byte("192.168.1.64:6667"))), //FIXME config?
		files,
		peers,
	})
	check(err)
	return b.Bytes()
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
