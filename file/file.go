package file

import (
	"gotorrent/bitfield"
	"gotorrent/torrentfile"
	"os"
	"path/filepath"
)

type File struct {
  File *os.File
  Downloaded bitfield.Bitfield
}

func New(path string, tf torrentfile.TorrentFile, resume bool) (*File, error) {

  var f *os.File
  var err error

  if !resume {
    f, err = AllocateFile(path, tf.Name, tf.Length)
    if err != nil {
      return nil, err
    }
  } else {

  }

  // read downloaded pieces and specify in bitfield
  var bitfield bitfield.Bitfield
  bitfield = make([]byte, 0)

  return &File{
    File: f,
    Downloaded: bitfield,
  }, nil

}

func WriteBufToFile(path, name string, buf []byte) (int, error) {
  f, err := os.Create(filepath.Join(path, filepath.Base(name))) 
  if err != nil {
    return 0, err
  }

  defer func() {
        if err := f.Close(); err != nil {
            panic(err)
        }
  }()

  bytesWritten, err := f.Write(buf)
  if err != nil {
    return 0, err
  }

  return bytesWritten, nil
}

// creates an allocated file the size of the file to be downloaded
func AllocateFile(path, name string, fileSize int) (*os.File, error) {

  f, err := os.Create(filepath.Join(path, filepath.Base(name))) 
  if err != nil {
    return nil, err
  }

  size := int64(fileSize)
  
  err = f.Truncate(size)
  if err != nil {
    return nil, err
  }
  return f, nil
}

func (f *File) WritePieceToFile(buf []byte, begin, end int) error {

  _, err := f.File.Seek(int64(begin), 0)
  if err != nil {
    return err
  }

  _, err = f.File.Write(buf)
  if err != nil {
    return err
  }

  return nil
}

func (f *File) ReadPiecesFromFile() error {
  return nil 
}

