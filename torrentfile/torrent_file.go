package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)

type bencodeInfo struct {
  Pieces string `bencode:"pieces"` 
  PieceLength int `bencode:"piece length"` 
  Length int `bencode:"length"` 
  Name string `bencode:"name"` 
}

type bencodeTorrent struct {
  Announce string `bencode:"announce"`
  Info bencodeInfo `bencode:"info"`
}


type Peer struct {
  IP net.IP
  Port uint16
}

type TorrentFile struct {
  Announce string
  PeerID [20]byte
  InfoHash [20]byte
  PieceHashes [][20]byte
  PieceLength int
  Length int 
  Name string 
}

type bencodeTrackerResponce struct {
  Interval int `bencode:"interval"`
  Peers string `bencode:"peers"`
}

func (i *bencodeInfo) hash() ([20]byte, error) {
  var buf bytes.Buffer
  err := bencode.Marshal(&buf, *i)
  if err != nil {
    return [20]byte{}, err
  }
  h := sha1.Sum(buf.Bytes())
  return h, nil
}


func (p *Peer) String() (s string) {
  return fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
}

// parses peers IP addresses and ports from a buffer 
func unmarshal(peersBin []byte) ([]Peer, error) {
  const peerSize = 6 // 4 for ip address and 2 for port
  numPeers := len(peersBin) / peerSize
  if len(peersBin) % peerSize != 0 {
    err := fmt.Errorf("Received malformed peers")
    return nil, err
  }
  // allocates an array of Peer structs dynamically based on numPeers
  peers := make([]Peer, numPeers)
  for i := 0; i < numPeers; i++ {
    offset := i * peerSize
    peers[i].IP = net.IP(peersBin[offset : offset + 4])
    peers[i].Port = binary.BigEndian.Uint16(peersBin[offset + 4: offset + 6])
  }

  return peers, nil
}

// pieces are pieces of a file that are hashed to ensure their validity as 
// we download them, and returns a dynamically allocated array of sha1 hashes
func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
  hashLen := 20
  //convert the bencode info  pieces attribute into a byte array
  buf := []byte(i.Pieces)
  if len(buf) % hashLen != 0 {
    err := fmt.Errorf("Received malformed pieces of length %d", len(buf))
    return nil, err
  }

  numhashes := len(buf) / hashLen
  hashes := make([][20]byte, numhashes)

  for i := 0; i < numhashes; i++ {
    copy(hashes[i][:], buf[i * hashLen:(i+1)*hashLen])
  }

  fmt.Printf("total number of hashes: %d\n", len(hashes))
  return hashes, nil
}

func (bto bencodeTorrent) toTorrentFile() (TorrentFile, error) {

  infohash, err := bto.Info.hash()
  if err != nil {
    return TorrentFile{}, err
  }
  
  pieceHashes, err := bto.Info.splitPieceHashes()
  if err != nil {
    return TorrentFile{}, err
  }

  var peerID [20]byte
	_, err = rand.Read(peerID[:])

  tf := TorrentFile{
    Announce: bto.Announce,
    PeerID: peerID,
    InfoHash: infohash,
    PieceHashes: pieceHashes,
    PieceLength: bto.Info.PieceLength,
    Length: bto.Info.Length,
    Name: bto.Info.Name,
  }

  tf.PrintTorrentFile()

  return tf, nil
}

// opens the torrent file and converts it into a struct
func Open(path string) (TorrentFile, error) {
  file, err := os.Open(path)
  if err != nil {
    return TorrentFile{}, err
  }
  defer file.Close()

  bto := bencodeTorrent{}
  err = bencode.Unmarshal(file, &bto)
  if err != nil {
    return TorrentFile{}, err
  }
  return bto.toTorrentFile()
}

func (t *TorrentFile) PrintTorrentFile() {
  fmt.Printf("Announce string: %s\nNum Pieces: %d\nPiece Length: %d\nLength: %d\nName: %s\nInfohash: %x\n", t.Announce, len(t.PieceHashes), t.PieceLength, t.Length, t.Name, string(t.InfoHash[:]))
}

func (t *TorrentFile) BuildTrackerUrl(port uint16) (string, error) {
  base, err := url.Parse(t.Announce)
  if err != nil {
    return "", err
  }
  params := url.Values {
    "info_hash": []string{string(t.InfoHash[:])},
    "peer_id": []string{string(t.PeerID[:])},
    "port": []string{strconv.Itoa(int(port))},
    "uploaded": []string{"0"},
    "downloaded": []string{"0"},
    "compact": []string{"1"},
    "left": []string{strconv.Itoa(int(t.Length))},
  }

  base.RawQuery = params.Encode()
  return base.String(), nil
}

func (t *TorrentFile) RequestPeers(port uint16) ([]Peer, error) {
  url, err := t.BuildTrackerUrl(port)
  if err != nil {
    return nil, err
  }

  // create a client with a timeout of 15 seconds
  client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
  if err != nil {
    return nil, err
  }

  defer resp.Body.Close()

  tracker := bencodeTrackerResponce{}

  err = bencode.Unmarshal(resp.Body, &tracker)

  if err != nil {
    return nil, err
  }

  return unmarshal([]byte(tracker.Peers))
}

