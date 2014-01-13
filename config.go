//Copyright Reed Allman 2013
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
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sync"
)

//actual metadata for file
type bt_file struct {
	Time         int64  `time`
	Length       int64  `length`
	Piece_length int    `piece_length`
	Pieces       string `pieces`
}

//uses local absolute path, generates metadata for file
func createFileMeta(path string, f os.FileInfo) (bt bt_file, err error) {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return bt, err
	}

	fmt.Println(f.Size())
	//TODO compute this smarter, not just min(256k, len(file))
	plength := int(math.Min(float64(PIECE_LENGTH), float64(f.Size())))

	if plength == 0 {
		return bt, err
	}
	iters := len(d) / plength
	if len(d)%plength > 0 {
		iters += 1
	}

	//compute sha1 of each piece
	var wg sync.WaitGroup
	pieces := make([]byte, 0, iters*20)
	wg.Add(iters)
	for i := 0; i < iters; i++ {
		go func(i int) {
			s := sha1.Sum(d[plength*i : int(math.Min(float64(plength*(i+1)), float64(len(d))))])
			pieces = append(pieces[:i*20], append(s[:], pieces[i*20:]...)...)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return bt_file{f.ModTime().Unix(), f.Size(), plength, string(pieces)}, nil
}

func loadShare(secret string, s *share) {
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
		s.Db = db
		shares[secret] = *s
		//shares[secret] = share{Secret: secret, Path: s.Path, Db: db, peers: s.peers}
		//s = shares[secret]
	}
	//drop some tables
	if newShare {
		//TODO potential bottleneck here, walk is slow -- but must be done somehow
		//  it appears there are about 20 ways to do io in stdlib
		//TODO time not in sync db, they parse bencoding -- maybe a good idea?
		//      allows: select * from files where time > x;  x = most recent, gets all new
		_, err := s.Db.Exec(
			`CREATE TABLE files (
          path TEXT NOT NULL PRIMARY KEY,
          data BLOB NOT NULL);`)
		check(err)
	}

	insert, err := s.Db.Prepare("INSERT INTO files(path, data) values(?, ?)")
	check(err)
	update, err := s.Db.Prepare("UPDATE files SET data = ? WHERE path = ?")
	check(err)

	filepath.Walk(s.Path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		relPath := path[len(s.Path)+1:] //slice abs + / off
		fmt.Println(relPath)

		updated := false
		if !newShare {
			btf, err := s.getFileMeta(relPath)
			check(err)

			if btf != nil && f.ModTime().Unix() > btf.Time {
				updated = true
			} else if btf != nil {
				return nil //if not update & we have something, we can leave
			}
			//if nil, new file.. so treat same
		}
		btf, err := createFileMeta(path, f)
		check(err)

		fmt.Println(path)

		var b bytes.Buffer
		err = bencode.Marshal(&b, btf)
		check(err)
		if updated {
			_, err = update.Exec(relPath, b.Bytes())
			check(err)
			fmt.Println("update")
		} else {
			_, err = insert.Exec(relPath, b.Bytes())
			fmt.Println("not update")
			check(err)
		}
		return nil
	})
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
			//TODO tricky here...
			s = newShare(secret, s.Path)
			loadShare(secret, &s)
		}
	}
}
