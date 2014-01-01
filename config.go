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
)

//uses local absolute path, generates metadata for file
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
		shares[secret] = share{Secret: secret, Path: s.Path, Db: db, peers: s.peers}
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
		stmt, err := db.Prepare("insert into files(path, time, data) values(?, ?, ?)")

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
			//TODO tricky here...
			s = newShare(secret, s.Path)
			loadShare(secret, s)
		}
	}
}
