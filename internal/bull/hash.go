package bull

import (
	"fmt"
	"hash"
	"hash/fnv"
)

func quickhash() hash.Hash {
	return fnv.New128()
}

// hashSum is a convenience function when you don’t want to use a bytes.NewReader
func hashSum(content []byte) string {
	h := quickhash()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}
