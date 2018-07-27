package fs

import (
	"io"
	"os"
	"path"

	"github.com/nameoffnv/httpfiles/storage"
	"github.com/pkg/errors"
)

type FileStorage struct {
	path string
}

func New(path string) storage.Storage {
	return &FileStorage{
		path: path,
	}
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

func (s *FileStorage) Save(filename string, data io.Reader) (int64, error) {
	if err := os.MkdirAll(path.Join(s.path, filename[:2]), os.ModePerm); err != nil {
		return 0, errors.Wrap(err, "mkdir")
	}

	f, err := os.OpenFile(path.Join(s.path, filename[:2], filename), os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return 0, errors.Wrap(err, "open file")
	}

	n, err := io.Copy(f, data)
	if err != nil {
		return 0, errors.Wrap(err, "write file")
	}

	return n, nil
}

func (s *FileStorage) Delete(id string) error {
	fname := path.Join(s.path, id[:2], id)
	if _, err := os.Stat(fname); err != nil {
		return storage.ErrNotFound
	}
	return os.Remove(fname)
}
