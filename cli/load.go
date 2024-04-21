package cli

import (
	"fmt"
  "math"
)


func ProgressBar(current, length int) {
  hash := rune('#')
  bar := "............................."

  ratio := float64(current) / float64(length)
  index := math.Round(ratio * float64(len(bar)))

  in := []rune(bar)

  for i := 0; i < int(index); i++ {
    in[i] = hash
  }
  bar = string(in)
  fmt.Printf("%.2f%% %s\r", ratio * 100.0, bar)
}
