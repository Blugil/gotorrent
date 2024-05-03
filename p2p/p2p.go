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
	Peers  []torrentfile.Peer
	PeerID [20]byte
	TF     torrentfile.TorrentFile
}

type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
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

func validatePiece(pieceHash [20]byte, buf []byte) (bool, error) {
	hash := sha1.Sum(buf)
	if !bytes.Equal(pieceHash[:], hash[:]) {
		return false, fmt.Errorf("Piece hash doesn't match torrent file\nExpected: %x, got: %x", pieceHash, hash)
	}
	return true, nil
}

func downloadPiece(c *client.Client, pw *pieceWork) (*pieceResult, error) {

	state := pieceProgress{
		index:      pw.index,
		client:     c,
		buf:        make([]byte, pw.length),
		downloaded: 0,
		requested:  0,
		backlog:    0,
	}

	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {

		if !state.client.Choked {
			blocksize := 16384 // max blocksize allowed to be requested
			for state.backlog < 5 && state.requested < pw.length {
				if pw.length-state.requested < blocksize {
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
		buf:   state.buf,
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

func (t Torrent) startDownload(peer torrentfile.Peer, pwQueue chan *pieceWork, prQueue chan *pieceResult) error {
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

		valid, err := validatePiece(pw.hash, pr.buf)
		if !valid {
			pwQueue <- pw
			return err
		}

		err = client.SendHave(pw.index)
		if err != nil {
			return err
		}

		cli.ProgressBar(cap(pwQueue)-len(pwQueue), cap(pwQueue))
		prQueue <- pr
	}

	return nil
}

func (t Torrent) DownloadTorrent(outPath, resumePath string, resume bool) error {

	pieceWorkQueue := make(chan *pieceWork, len(t.TF.PieceHashes))
	pieceResultQueue := make(chan *pieceResult, len(t.TF.PieceHashes))

	var f *file.File
	var err error

	// for resuming a download
	if !resume {
		fmt.Println("Starting torrent...")
		f, err = file.New(outPath, t.TF)
		if err != nil {
			return err
		}

		for index, pieceHash := range t.TF.PieceHashes {
			pieceLength := t.calculatePieceSize(index)
			pieceWorkQueue <- &pieceWork{
				index:  index,
				hash:   pieceHash,
				length: pieceLength,
			}
		}
	} else {
		fmt.Println("Continuing torrent...")
		f, err = file.Open(resumePath)
		if err != nil {
			return err
		}
		for index, pieceHash := range t.TF.PieceHashes {
			pieceLength := t.calculatePieceSize(index)
			pieceBuffer, err := f.ReadPieceFromFile(t.calcPieceBounds(index))
			if err != nil {
				return err
			}
			valid, err := validatePiece(pieceHash, pieceBuffer)
			// add the piece to the queue if it contains an invalid hash (isn't in the file)
			// probably pretty expensive, but validates against malicious byte injection into an empty file
			// could just validate against 0's which is what the partial file should have instead of
			// actual data due to the truncate
			if !valid {
				pieceWorkQueue <- &pieceWork{
					index:  index,
					hash:   pieceHash,
					length: pieceLength,
				}
			}
		}
		if len(pieceWorkQueue) == 0 {
			return fmt.Errorf("Selected file is already a valid download of this torrent")
		}
		fmt.Printf("There are %d pieces remaining to download.\n", len(pieceWorkQueue))
	}

	defer func() {
		if err := f.File.Close(); err != nil {
			panic(err)
		}
	}()

	numPiecesToDownload := len(pieceWorkQueue)

	for _, peer := range t.Peers {
		go t.startDownload(peer, pieceWorkQueue, pieceResultQueue)
	}

	// create a file the size of the torrent

	donePieces := 0
	for donePieces < numPiecesToDownload {
		result := <-pieceResultQueue
		begin, end := t.calcPieceBounds(result.index)
		err := f.WritePieceToFile(result.buf, begin, end)
		if err != nil {
			return err
		}
		donePieces++
	}

  fmt.Println()

	close(pieceWorkQueue)

	fmt.Println("Pieces written to file:", donePieces)
	fmt.Println("Successfully downloaded the torrent")

	if err != nil {
		return err
	}
  return nil
}
