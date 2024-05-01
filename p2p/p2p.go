package p2p

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"gotorrent/cli"
	"gotorrent/client"
	"gotorrent/file"
	"gotorrent/message"
	"gotorrent/torrentfile"
	"time"
)

type Torrent struct {
  Peers []torrentfile.Peer
  PeerID [20]byte
  TF torrentfile.TorrentFile
}

type pieceProgress struct {
  index int
  client *client.Client
  buf []byte
  downloaded int
  requested int
  backlog int
}

type pieceWork struct {
  index int
  hash [20]byte
  length int
}

type pieceResult struct {
  index int
  buf []byte
}

func (state *pieceProgress) readMessage() error {
  msg, err := message.Read(state.client.Conn)
  if err != nil {
    return err
  }
  if msg == nil {
    return nil
  }
  switch msg.ID {
    case message.MsgUnchoke:
      state.client.Choked = false
    case message.MsgChoke:
      state.client.Choked = true
    case message.MsgHave:
      index, err := msg.ParseHavePiece(msg)
      if err != nil {
        return err
      }
      state.client.Bitfield.SetPiece(index)
    case message.MsgPiece:
      n, err := msg.ParsePiece(state.index, state.buf, msg)
      if err != nil {
        return err
      }
      state.downloaded += n
      state.backlog--
  }

  return nil
}

func  validatePiece(pw *pieceWork, buf []byte) error {
  hash := sha1.Sum(buf)
  if !bytes.Equal(pw.hash[:], hash[:]) {
    return fmt.Errorf("Piece hash doesn't match torrent file\nExpected: %x, got: %x", pw.hash, hash) 
  }
  return nil
}

func downloadPiece(c *client.Client, pw *pieceWork) (*pieceResult, error) {

  state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
    downloaded: 0,
    requested: 0,
    backlog: 0,
	}

  c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{}) 

  for state.downloaded < pw.length {

    if !state.client.Choked {
      blocksize := 16384 // max blocksize allowed to be requested
      for state.backlog < 5 && state.requested < pw.length {
        if pw.length - state.requested < blocksize {
          blocksize = pw.length - state.requested
        }
        // start asking for blocks
        err := c.SendRequest(pw.index, state.requested, blocksize)
        if err != nil {
          return nil, err
        }
        state.backlog++ 
        state.requested += blocksize
      }
    }
    
    err := state.readMessage()

    if err != nil {
      return nil, err
    }
  }

  return &pieceResult{
    index: pw.index,
    buf: state.buf,
  }, nil
}


func (t Torrent) calcPieceBounds(index int) (begin, end int) {
  begin = index * t.TF.PieceLength
  end = begin + t.TF.PieceLength

  if end > t.TF.Length {
    end = t.TF.Length
  }

  return begin, end
}

func (t Torrent) calculatePieceSize(index int) int {
  begin, end := t.calcPieceBounds(index)
  return end - begin
}

func (t Torrent) StartDownload(peer torrentfile.Peer, pwQueue chan *pieceWork, prQueue chan *pieceResult) error {
  client, err := client.New(peer, t.PeerID, t.TF.InfoHash)
  if err != nil {
    fmt.Printf("Could not handshake with peer %s, disconnecting\n", peer.String())
    return err
  }

  defer client.Conn.Close()

  client.SendUnchoke()
  client.SendInterested()

  for pw := range pwQueue {
    if !client.Bitfield.HasPiece(pw.index) {
      pwQueue <- pw
      continue
    }

    pr, err := downloadPiece(client, pw)
    if err != nil {
      pwQueue <- pw
      return err
    }

    err = validatePiece(pw, pr.buf)
    if err != nil {
      pwQueue <- pw
      return err
    }

    err = client.SendHave(pw.index)
    if err != nil {
      return err
    }

    //client.Bitfield.SetPiece(pw.index)

    cli.ProgressBar(cap(pwQueue) - len(pwQueue), cap(pwQueue))
    prQueue <- pr
  }

  return nil
}


func (t Torrent) DownloadTorrent(outPath string) error {
  pieceWorkQueue := make(chan *pieceWork, len(t.TF.PieceHashes))
  pieceResultQueue := make(chan *pieceResult, len(t.TF.PieceHashes))

  for index, peerHash := range t.TF.PieceHashes {
    pieceLength := t.calculatePieceSize(index)
    pieceWorkQueue <- &pieceWork{
      index: index,
      hash: peerHash,
      length: pieceLength,
    }
  }

  for _, peer := range t.Peers {
    go t.StartDownload(peer, pieceWorkQueue, pieceResultQueue)
  }

  // create a file the size of the torrent
  f, err := file.New(outPath, t.TF, false)
  if err != nil {
    return err
  }

  defer func() {
        if err := f.File.Close(); err != nil {
            panic(err)
        }
  }()

  // for each piece that finishes, write it into the correct bounds

  //finalFile := make([]byte, t.TF.Length)

  donePieces := 0
  for donePieces < len(t.TF.PieceHashes) {
    result := <- pieceResultQueue
    begin, end := t.calcPieceBounds(result.index)
    err = f.WritePieceToFile(result.buf, begin, end)
    if err != nil {
      return err
    }
    donePieces++
  }
  close(pieceWorkQueue)

  fmt.Printf("\npieces copied: %d\n", donePieces)
  fmt.Println("Successfully downloaded the torrent")
  
  //_, err := file.WriteBufToFile(outPath, t.TF.Name, finalFile)
  if err != nil {
    return err
  }
  return nil
}
