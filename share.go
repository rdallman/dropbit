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
	//"os"
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
		dap := s.createPiece(r.File, -1, -1, data)
		fmt.Println("meta len", len(dap))
		return dap
	}

	buf := make([]byte, r.Length)
	//f, err := os.Open(s.Path + "/" + r.File)
	//check(err)
	//_, err = f.ReadAt(buf, int64(r.Index*mdata.Piece_length+r.Begin))
	//f.Close()
	//check(err)

	return s.createPiece(r.File, r.Index, r.Begin, buf)
}

func (s *share) processPiece(u UDPMessage, out chan UDPMessage) {
	msg, _ := u.data, u.addr
	fmt.Println("Process piece")
	var p Piece
	err := bencode.Unmarshal(bytes.NewBuffer(msg), &p)
	check(err)

	var data []byte
	err = s.Db.QueryRow("SELECT data FROM files WHERE path = ?", p.File).Scan(&data)

	//if meta and file we're not aware of, just take their meta
	if err == sql.ErrNoRows && p.Index == -1 && p.Begin == -1 {
		_, err := s.Db.Exec("INSERT INTO files(path, data) values(?, ?)", p.File, p.Piece)
		check(err)
	}

	var mdata bt_file
	err = bencode.Unmarshal(bytes.NewBuffer(data), &mdata)
	check(err) //as long as there's no panic here, we're good

	if p.Index == -1 && p.Begin == -1 {
		fmt.Println("got meta")
		var ydata bt_file
		err = bencode.Unmarshal(bytes.NewBuffer([]byte(p.Piece)), &ydata)
		check(err)

		if mdata.Time > ydata.Time {
			return //I don't need this
		}

		//process meta; at this point, I either didn't have it or theirs is newer

		rlength := int(math.Min(float64(BLOCK_SIZE), float64(ydata.Piece_length)))
		for i := 0; i < len(ydata.Pieces)/20; i++ { //256k chunks
			pb := i * 20 //piece hash actual index
			fmt.Println(len(ydata.Pieces)/20, len(mdata.Pieces)/20)
			if i >= len(mdata.Pieces)/20 ||
				mdata.Pieces[pb:pb+20] != ydata.Pieces[pb:pb+20] {
				for j := 0; j < ydata.Piece_length; j += rlength { //16K chunks
					length := int(math.Min(float64(ydata.Length-int64((i*ydata.Piece_length)+j)), float64(rlength)))
					fmt.Println(length)
					//out <- UDPMessage{sender, s.createRequest(p.File, i, j, length)}
					fmt.Println("want meta for", p.File, i, j, length)
					if length < rlength {
						break
					}
				}
			}
		}
	} else if data != nil {
		fmt.Printf("opening %s to write at %d, %d\n", p.File, p.Index, p.Begin)
		//TODO eh, flags are weird
		//TODO also need to see behavior on files where WriteAt() will be OOB
		//f, err := os.OpenFile(s.Path+"/"+p.File, os.O_RDWR|os.O_CREATE, 0666)
		//check(err)
		//_, err = f.WriteAt([]byte(p.Piece), int64(p.Index*mdata.Piece_length+p.Begin))
		//f.Close()
		//check(err)
	}
}

func (s *share) createPing(secret string) []byte {
	buf := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(buf, Header{
		"ping",
		*port,
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
		*port,
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
		*port,
		fmt.Sprintf("%s", sha1.Sum([]byte(s.Secret))),
		fmt.Sprintf("%s", sha1.Sum([]byte("192.168.1.64:6667"))),
		path,
		index,
		begin,
		fmt.Sprintf("%s", piece),
	})
	check(err)
	return b.Bytes()
}

func (s *share) createRequest(path string, index, begin, length int) []byte {
	b := bytes.NewBuffer([]byte("DBIT"))
	err := bencode.Marshal(b, Request{
		"req",
		*port,
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
