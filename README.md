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
SHAKE (list of [file, meta], port, id, peers)
REQUEST (file, offset, piece offset)
REPLY (file, offset, piece offset, real data) ... this might could reduce to just data

basically each file has a .torrent file as defined here <http://www.bittorrent.org/beps/bep_0003.html> + timestamp
when they shake, hash each to figure out any differences
  if != && our stamp < theirs
    save their meta (0 haves) && request


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




