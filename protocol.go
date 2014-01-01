package main

import ()

type Header struct {
	M     string `m`
	Port  int    `port`
	Share string `share`
	Peer  string `peer`
}

type Shake struct {
	M     string            `m`
	Port  int               `port`
	Share string            `share`
	Peer  string            `peer`
	Files map[string]string `files` //map[filename]hash of info
	Peers []string          `peers`
}

//TODO associate port w/ file? leave open until everybody who needs it
//is done and then move on to the next file? just open multiple ports at a time...

//request with index, begin & length set to -1 requests torrent for File
type Request struct {
	M      string `m`
	Port   int    `port`
	Share  string `share`
	Peer   string `peer`
	File   string `file`
	Index  int64  `index` //offset of piece
	Begin  int64  `begin` //offset w/i piece
	Length int64  `offset`
}

//piece with index & being set to -1 means Piece is a .torrent for File
//TODO when updated file JUST SEND THIS, EL CHEAPO
type Piece struct {
	M     string `m`
	Port  int    `port`
	Share string `share`
	Peer  string `peer`
	File  string `file`
	Index int64  `index`
	Begin int64  `begin`
	Piece []byte `piece`
}
