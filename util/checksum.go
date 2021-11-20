package util

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
)

func ValidateChecksum(p string, c string) (bool, error) {
	if p == "" || c == "" {
		return false, &ErrorString{S: "Package or checksum doesn't exist"}
	}

	file, err := ioutil.ReadFile(p)
	if err != nil {
		return false, nil
	}

	checksum_byte := sha256.Sum256([]byte(file))
	checksum := hex.EncodeToString(checksum_byte[:])

	return checksum == c, nil
}
