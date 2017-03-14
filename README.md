[![Build Status](https://drone.io/github.com/roberthorn/dropbit/status.png)](https://drone.io/github.com/roberthorn/dropbit/latest)
[![Coverage Status](https://coveralls.io/repos/roberthorn/dropbit/badge.png)](https://coveralls.io/r/roberthorn/dropbit)

# The Big To-Do List

### meow

* known peers at share level vs. server level (where to keep conn open?)
* watch files for changes, on start check if file has changed (mod time)
* haves (bitset). implement per peer per file piece (we need to go deeper)
* peer discovery

### later

* key generation
* encrypt / decrypt message w/ secret ... then session keys
* web interface
* uTP (cgo?)

* name ideas: bitlocker, torrsync, bitsafe, bitpound
* see what happens when editing same file in 2 places


### protocol rambling

basically each file has a .torrent file as defined here <http://www.bittorrent.org/beps/bep_0003.html> + timestamp
when they shake, hash each to figure out any differences
  if != && our stamp < theirs
    save their meta (0 haves) && request

rambling v2:

step 1:
(ping)
!known peer
  request [file, hash]..., [peer]...

step 2:
(with meta)
for file, hash:
  !known files || theirs != ours
    request(file, -1, -1, -1)
for peer:
  !known peer
    request [file, hash]..., [peer]...

step 3:
(with file meta)
mfile = SELECT file
for piece:
  mpiece != piece
    request(piece)

sample:

```
(discovery)
Shake         -> <-       Shake
RequestMeta   ->
RequestMeta   ->          (can also RequestMeta)
...
                 <-       PieceMeta
                 <-       PieceMeta
                 ...
Request       ->
Request       ->
...
                 <-       Reply
                 <-       Reply
                 ...
Have/Cancel   ->
Have/Cancel   ->
...
```

##### Caveats:

After shake, there doesn't have to particularly be any order on a per file
basis, and pieces don't have to come in any order either

RequestMeta is a Request where the parameters for index, begin and length
are all set to -1 and will get the full (updated) ".torrent" info for a file

PieceMeta is a Piece in reponse to a RequestMeta where the index and begin are
set to -1 and the Piece field will contain the ".torrent" for a given file




