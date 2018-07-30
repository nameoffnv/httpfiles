package memory

import (
	"bytes"
	"fmt"
	"hash"
	"io"
)

type objectWriter struct {
	data     []byte
	h        hash.Hash
	postSave func(string, []byte)
}

func (w *objectWriter) Write(b []byte) (int, error) {
	w.data = append(w.data, b...)
	return len(b), nil
}

func (w *objectWriter) Size() int64 {
	return int64(len(w.data))
}

func (w *objectWriter) Save() (string, error) {
	if _, err := io.Copy(w.h, bytes.NewReader(w.data)); err != nil {
		return "", err
	}
	hs := fmt.Sprintf("%x", w.h.Sum(nil))
	w.postSave(hs, w.data)
	return hs, nil
}

func (w *objectWriter) Remove() error {
	return nil
}
