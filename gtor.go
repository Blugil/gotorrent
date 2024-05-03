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

  fmt.Println("in path is: ", *inPath)
  fmt.Println("out path is: ", *outPath)
  fmt.Println("resume path is: ", *resumePath)

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
  
  fmt.Println("resume:", resume)

  err = t.DownloadTorrent(*outPath, resume)
  if err != nil {
    fmt.Println(err)
  }
  
}
