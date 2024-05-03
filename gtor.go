package main

import (
	"flag"
	"fmt"
	"gotorrent/p2p"
	"gotorrent/torrentfile"
)

func main() {

  inPath := flag.String("t", "", "torrent file for download")
  outPath := flag.String("o", ".", "the download output path")
  resumePath := flag.String("r", "", "input partial download gtor file to resume")


  flag.Parse()

  tf, err := torrentfile.Open(*inPath)
  if err != nil {
    panic(err)
  }

  peers, err := tf.RequestPeers()


  t := p2p.Torrent {
    Peers: peers,
    PeerID: tf.PeerID,
    TF: tf,
  }

  if *inPath == "" {
    panic(fmt.Errorf("No input file passed in"))
  }

  resume := *resumePath != ""

  err = t.DownloadTorrent(*outPath, *resumePath, resume)
  if err != nil {
    fmt.Println(err)
  }
}
