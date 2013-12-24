# The Big To-Do List

* figure out read-only vs full-access (LAN appears to only search for 1 key,
  which is neither of these as far as I can tell)
* known hosts
* encrypt / decrypt message w/ secret ... then session keys
* is my file out of date? ... then fix that


* key generation
* web interface
* uTP (cgo?)
* use secret to get hosts to render web page to read data remotely (web)?

* name ideas: bitlocker, torrsync, bitsafe, bitpound
* see what happens when editing same file in 2 places


go listenforpeers() {
  go LANpeers()
  go NATpeers()
}

you: I have these secrets, talk to me on <ip>:<port>
PING
me: hey, I have that 
PING
SAVE KEY HERE (instance of share), in separate thread start:
  you: okay, here's my meta
  SHAKE
  me: okay, here's my meta
  SHAKE
  you: okay, I need this (<- shoot off lots of requests)
  REQUESTS
  me: okay, I need this (<- shoot off lots of requests)
  REQUESTS

  ...REPLIES
  ...REPLIES


so listen for pings
 we get one, get the share[secret]
  go c.Talk()
