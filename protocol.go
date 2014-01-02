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

//request with index, begin & length set to -1 requests torrent for File
type Request struct {
	M      string `m`
	Port   int    `port`
	Share  string `share`
	Peer   string `peer`
	File   string `file`
	Index  int    `index` //offset of piece
	Begin  int    `begin` //offset w/i piece
	Length int    `offset`
}

//piece with index & being set to -1 means Piece is a .torrent for File
//TODO when updated file JUST SEND THIS, EL CHEAPO
type Piece struct {
	M     string `m`
	Port  int    `port`
	Share string `share`
	Peer  string `peer`
	File  string `file`
	Index int    `index`
	Begin int    `begin`
	Piece []byte `piece`
}
