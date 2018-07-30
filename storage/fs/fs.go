package fs

import (
	"io"
	"os"
	"path"

	"hash"

	"github.com/nameoffnv/httpfiles/storage"
	"github.com/pkg/errors"
)

type FileStorage struct {
	path     string
	hashFunc func() hash.Hash
}

func New(path string, hashFunc func() hash.Hash) storage.Storage {
	return &FileStorage{
		path:     path,
		hashFunc: hashFunc,
	}
}

func (s *FileStorage) NewObjectWriter() (storage.ObjectWriter, error) {
	return NewObjectWriter(s.path, s.hashFunc())
}

func (s *FileStorage) Get(id string) (io.ReadCloser, error) {
	fname := path.Join(s.path, id[:2], id)
	if _, err := os.Stat(fname); err != nil {
		return nil, storage.ErrNotFound
	}

	f, err := os.Open(fname)
	if err != nil {
		return nil, errors.Wrap(err, "open file")
	}

	return f, nil
}

func (s *FileStorage) Delete(id string) error {
	fname := path.Join(s.path, id[:2], id)
	if _, err := os.Stat(fname); err != nil {
		return storage.ErrNotFound
	}
	return os.Remove(fname)
}
