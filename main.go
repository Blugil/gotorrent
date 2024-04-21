package main

import (
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

  peers, err := tf.RequestPeers(6969)

  t := p2p.Torrent {
    Peers: peers,
    PeerID: tf.PeerID,
    TorrentFile: tf,
  }
  
  // init queues

  //cli.ProgressBar(100,200)
  t.DownloadTorrent(outPath)


  //}
}
