package main

import (
	"bytes"
	bencode "code.google.com/p/bencode-go"
	"crypto/sha1"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"math"
	"net"
	"os"
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

func (s *share) processMeta(u UDPMessage, out chan UDPMessage) {
	msg, sender := u.data, u.addr
	fmt.Println("process meta")
	mfiles := s.getFileHashes()

	var shake Shake
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &shake)
	check(err)

	yfiles := shake.Files

	for yf, yhash := range yfiles {
		mhash, ok := mfiles[yf]
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

	var data []byte
	err = s.Db.QueryRow("SELECT data FROM files WHERE path = ?", r.File).Scan(&data)
	check(err)

	var mdata bt_file
	err = bencode.Unmarshal(bytes.NewBuffer(data), &mdata)
	check(err)

	if r.Index == -1 && r.Begin == -1 && r.Length == -1 {
		return s.createPiece(r.File, -1, -1, data)
	}

	buf := make([]byte, r.Length)
	f, err := os.Open(r.File)
	check(err)
	_, err = f.ReadAt(buf, int64(r.Index*mdata.Piece_length+r.Begin))
	check(err)
	return buf
}

func (s *share) processPiece(u UDPMessage, out chan UDPMessage) {
	msg, sender := u.data, u.addr
	fmt.Println("Process piece")
	var p Piece
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &p)
	check(err)

	var data []byte
	err = s.Db.QueryRow("SELECT data FROM files WHERE path = ?", p.File).Scan(&data)
	check(err)

	var mdata bt_file
	err = bencode.Unmarshal(bytes.NewBuffer(data), &mdata)
	check(err)

	if p.Index == -1 && p.Begin == -1 {
		fmt.Println("got meta")
		//well as much as I wanted to get it out of the way, it sure is ugly
		s.processFileMeta(p.File, mdata, p.Piece, sender, out)
	} else {
		fmt.Printf("opening %s to write at %d, %d\n", p.File, p.Index, p.Begin)
		//TODO eh, flags are weird
		//TODO also need to see behavior on files where WriteAt() will be OOB
		f, err := os.OpenFile(p.File, os.O_RDWR|os.O_CREATE, 0666)
		check(err)
		_, err = f.WriteAt(p.Piece, int64(p.Index*mdata.Piece_length+p.Begin))
		check(err)
	}
}

//I got a fever, and the only prescription, is more parameters...
func (s *share) processFileMeta(file string, mdata bt_file, rec_meta []byte, sender *net.UDPAddr, out chan UDPMessage) {
	var ydata bt_file
	b := bytes.NewBuffer(rec_meta)
	err := bencode.Unmarshal(b, &ydata)
	check(err)

	if mdata.Time < ydata.Time {
		//TODO save time, piece length, put all (off) "have" to 0, request each piece
		_, err = s.Db.Exec("UPDATE files SET data=? WHERE path = ?", b.Bytes(), file)
		check(err)
		for i := 0; i < len(ydata.Pieces); i += 20 { //256k chunks
			fmt.Println("yo pieces", ydata.Pieces[i:20])
			if i > len(mdata.Pieces)+20 ||
				mdata.Pieces[i:20] != ydata.Pieces[i:20] ||
				ydata.Piece_length != mdata.Piece_length { //TODO eh, maybe another conditional. why not?

				rlength := int(math.Min(float64(BLOCK_SIZE), float64(ydata.Piece_length)))
				for j := 0; j < ydata.Piece_length; j += rlength { //16K chunks
					length := int(math.Min(float64(ydata.Length), float64(j)))
					out <- UDPMessage{sender, s.createRequest(file, i, j, length)}
				}
			}
		}
	} else {
		//uh, send this their way? they should decide the above...
	}
	fmt.Println("done meta")
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

func (s *share) createPiece(path string, index, begin int, piece []byte) []byte {
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

func (s *share) createRequest(path string, index, begin, length int) []byte {
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
