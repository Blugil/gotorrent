package file

import (
	"gotorrent/torrentfile"
	"os"
	"path/filepath"
)

type File struct {
  File *os.File
}

func New(path string, tf torrentfile.TorrentFile) (*File, error) {

  f, err := AllocateFile(path, tf.Name, tf.Length)
  if err != nil {
    return nil, err
  }

  return &File{
    File: f,
  }, nil

}

func Open(path string) (*File, error) {
  f, err := os.OpenFile(path, os.O_RDWR, 0644)
  if err != nil {
    return nil, err
  }
  //check extension to gtor 

  return &File{
    File: f,
  }, nil
}

func AllocateFile(path, name string, fileSize int) (*os.File, error) {
  f, err := os.Create(filepath.Join(path, filepath.Base(name + ".gtor"))) 
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

func (f *File) ReadPieceFromFile(begin, end int) ([]byte, error) {
  buf := make([]byte, end-begin)
  _, err := f.File.Seek(int64(begin), 0)
  if err != nil {
    return nil, err
  }

  _, err = f.File.Read(buf)
  if err != nil {
    return nil, err
  }
  return buf, nil
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

