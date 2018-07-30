package redis_fs

import (
	"github.com/nameoffnv/httpfiles/storage"
)

type objectWriter struct {
	storage.ObjectWriter

	postSave func(string, int64) error
}

func (w *objectWriter) Save() (string, error) {
	h, err := w.ObjectWriter.Save()
	if err != nil {
		return "", err
	}

	if w.postSave != nil {
		if err := w.postSave(h, w.Size()); err != nil {
			return "", err
		}
	}

	return h, nil
}
