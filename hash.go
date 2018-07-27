package httpfiles

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"

	"github.com/pkg/errors"
)

var HashFuncs = map[string]HashFunc{
	"sha1":   HashSHA1,
	"sha256": HashSHA256,
	"md5":    HashMD5,
}

type HashFunc func(io.Reader) (string, error)

func HashSHA1(data io.Reader) (string, error) {
	return calculateHash(sha1.New(), data)
}

func HashSHA256(data io.Reader) (string, error) {
	return calculateHash(sha256.New(), data)
}

func HashMD5(data io.Reader) (string, error) {
	return calculateHash(md5.New(), data)
}

func ValidateHashes(data []byte, hashes map[string]string) error {
	for k, v := range hashes {
		fn, ok := HashFuncs[k]
		if !ok {
			continue
		}

		h, err := fn(bytes.NewReader(data))
		if err != nil {
			return err
		}

		if h != v {
			return errors.Errorf("%s hash mismatch", k)
		}
	}
	return nil
}

func calculateHash(h hash.Hash, data io.Reader) (string, error) {
	if _, err := io.Copy(h, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
