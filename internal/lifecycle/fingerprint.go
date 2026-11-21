package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
)

func Fingerprint(inputs ...[]byte) string {
	hash := sha256.New()
	for _, input := range inputs {
		hash.Write([]byte{0})
		hash.Write(input)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
