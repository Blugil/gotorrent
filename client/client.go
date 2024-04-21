package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gotorrent/bitfield"
	"gotorrent/handshake"
	"gotorrent/message"
	"gotorrent/torrentfile"
	"net"
	"time"
)

type Client struct {
	Conn        net.Conn
	Choked      bool
	Interested  bool
	Choking     bool
	Interesting bool
	Bitfield    bitfield.Bitfield
	peer        torrentfile.Peer
	infohash    [20]byte
	peerID      [20]byte
}

func handshakeWithPeer(conn net.Conn, peerID [20]byte, infohash [20]byte, peer torrentfile.Peer) (*handshake.HandShake, error) {
	h := handshake.HandShake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infohash,
		PeerID:   peerID,
	}

	//fmt.Printf("Sending handshake: %s\n", peer.String())
	req := h.Serialize()
	_, err := conn.Write(req)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("Receiving handshake: %s\n", peer.String())
	res, err := handshake.Read(conn)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(res.InfoHash[:], infohash[:]) {
		return nil, fmt.Errorf("expected hash: %x, but got hash: %x instead", infohash, res.InfoHash)
	}

	return res, nil
}

func recBitfield(conn net.Conn) (bitfield.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

	msg, err := message.Read(conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		err := fmt.Errorf("Expected bitfield but got %x", msg)
		return nil, err
	}
	if msg.ID != message.MsgBitfield {
		err := fmt.Errorf("Expected bitfield but got ID %d", msg.ID)
		return nil, err
	}

	return msg.Payload, nil
}

func New(peer torrentfile.Peer, peerID, infohash [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port), 10*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = handshakeWithPeer(conn, peerID, infohash, peer)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// receives the bitfield from the peer
	bf, err := recBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:        conn,
		Choked:      true,
		Interested:  false,
		Choking:     true,
		Interesting: false,
		Bitfield:    bf,
		peer:        peer,
		infohash:    infohash,
		peerID:      peerID,
	}, nil
}

func (c *Client) KeepAlive() error {
	message := message.Message{}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}
	//fmt.Printf("sent keepalive to peer: %s\n", c.peer.IP.String())
	return nil
}

func (c *Client) SendUnchoke() error {
	message := message.Message{
		ID: message.MsgUnchoke,
	}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}
	//fmt.Printf("sent unchoke to peer: %s\n", c.peer.IP.String())
	return nil
}

func (c *Client) SendChoke() error {
	message := message.Message{
		ID: message.MsgChoke,
	}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}
	//fmt.Printf("sent choke to peer: %s\n", c.peer.IP.String())
	return nil
}

func (c *Client) SendInterested() error {
	message := message.Message{
		ID: message.MsgInterested,
	}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}

	c.Interested = true
	//fmt.Printf("sent interested to peer: %s\n", c.peer.IP.String())
	return nil
}

func (c *Client) SendUninterested() error {
	message := message.Message{
		ID: message.MsgNotInterested,
	}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}

	c.Interested = true
	//fmt.Printf("sent uninterested to peer: %s\n", c.peer.IP.String())
	return nil
}

func (c *Client) SendRequest(index, begin, length int) error {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	message := message.Message{
		ID:      message.MsgRequest,
		Payload: payload,
	}

	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SendHave(index int) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload[:], uint32(index))
	message := message.Message{
		ID:      message.MsgHave,
		Payload: payload,
	}
	_, err := c.Conn.Write(message.Serialize())
	if err != nil {
		return err
	}
	return nil
}
