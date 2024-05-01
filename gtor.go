package main

import (
	"fmt"
	"gotorrent/p2p"
	"gotorrent/torrentfile"
	"os"
)

func main() {

  args := os.Args[1:]
  inPath := args[0]
  outPath := args[1]

  tf, err := torrentfile.Open(inPath)
  if err != nil {
    panic(err)
  }

  peers, err := tf.RequestPeers()

  t := p2p.Torrent {
    Peers: peers,
    PeerID: tf.PeerID,
    TF: tf,
  }
  
  err = t.DownloadTorrent(outPath)
  if err != nil {
    fmt.Println(err)
  }
  
}
