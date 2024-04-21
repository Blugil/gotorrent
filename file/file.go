package file

import "os"

func WriteBufToFile(path, name string, buf []byte) error {
  err := os.WriteFile(path + "/" + name, buf, 0644)
  if err != nil {
    return err
  }
  return nil
}
