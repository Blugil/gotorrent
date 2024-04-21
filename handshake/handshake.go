package handshake

import (
	"fmt"
	"io"
)


type HandShake struct {
  Pstr string
  InfoHash [20]byte
  PeerID [20]byte
}


// turns a handshake struct into a byte array for transmitting over tcp
func (h *HandShake) Serialize() []byte {
  buf := make([]byte, len(h.Pstr) + 49)
  buf[0] = byte(len(h.Pstr))
  curr := 1
  curr += copy(buf[curr:], h.Pstr)
  curr += copy(buf[curr:], make([]byte, 8))
  curr += copy(buf[curr:], h.InfoHash[:])
  curr += copy(buf[curr:], h.PeerID[:])

  return buf
}

func Read(r io.Reader) (*HandShake, error) {
  lengthBuf := make([]byte, 1)

  _, err := io.ReadFull(r, lengthBuf)
  if err != nil {
    return nil, err
  }

  pstrlen := int(lengthBuf[0])

  if pstrlen == 0 {
    err := fmt.Errorf("pstrlen cannot be 0")
    return nil, err
  }

  // 20 bytes for infohash, 20 bytes for peer id, 8 reserved bytes
  handshakebuf := make([]byte, pstrlen + 48) 
  _, err = io.ReadFull(r, handshakebuf)
  if err != nil {
    return nil, err
  }

  var infohash, peerID [20]byte

  // copying the correct bytes into the buffers
  copy(infohash[:], handshakebuf[pstrlen + 8: pstrlen + 28])
  copy(peerID[:], handshakebuf[pstrlen + 28:])


  handshake := HandShake{
    Pstr: string(handshakebuf[:pstrlen]),
    InfoHash: infohash,
    PeerID: peerID,
  }

  return &handshake, nil
}
