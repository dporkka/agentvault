package fileversion

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
