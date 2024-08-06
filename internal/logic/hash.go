package logic

import (
	"crypto/sha256"
	"fmt"
)

/*
returns a sha256 hash of the input string
*/
func HashString(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum)
}
