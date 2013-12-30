# The Big To-Do List

### meow

* figure out read-only vs full-access (LAN appears to only search for 1 key,
  which is neither of these as far as I can tell)
* known hosts
* encrypt / decrypt message w/ secret ... then session keys
* is my file out of date? ... then fix that


### later

* key generation
* web interface
* uTP (cgo?)
* use secret to get hosts to render web page to read data remotely (web)?

* name ideas: bitlocker, torrsync, bitsafe, bitpound
* see what happens when editing same file in 2 places


### protocol(ish) in human language
you: I have this secret, talk to me on <ip>:<port> (for peer discovery (LAN/WAN same?)
PING 

me: hey, I have that (upon any peer discovery)
PING

  SAVE KEY HERE (instance of share), in separate thread start:
  you: okay, here's my meta
  SHAKE 
  me: okay, here's my meta
  SHAKE 
  you: okay, I need this (<- shoot off lots of requests)
  REQUEST 
  me: okay, I need this (<- shoot off lots of requests)
  REQUEST

  ...REPLIES
  ...REPLIES

PING (id hash, secret hash, port)
SHAKE (list of [file, hash(meta)], peers)
REQUEST (file, offset, piece offset)
REPLY (file, offset, piece offset, real data) ... this might could reduce to just data

basically each file has a .torrent file as defined here <http://www.bittorrent.org/beps/bep_0003.html> + timestamp
when they shake, hash each to figure out any differences
  if != && our stamp < theirs
    save their meta (0 haves) && request


for each? (each should get a ping, and then meta -- in theory):
  him TO MULTICAST: KNOWS NOBODY
    write PING
  me TO ADDR FROM MULTICAST: I WANT TO KNOW HIM
    write PING (to him)
    -> read metadata
  him LISTENING ON REAL PORT: I WANT TO KNOW HIM
    read PING
    write metadata
    -> read meta

anyone w/ new metadata
  what do I need?



readMeta() 
  hash(youMeta) != hash(meMeta) 
    Unmarshal(youMeta)


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

##### Caveats:

After shake, there doesn't have to particularly be any order on a per file
basis, and pieces don't have to come in any order either

RequestMeta is a Request where the parameters for index, begin and length
are all set to -1 and will get the full (updated) ".torrent" info for a file

PieceMeta is a Piece in reponse to a RequestMeta where the index and begin are
set to -1 and the Piece field will contain the ".torrent" for a given file




