package fs

import (
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/nameoffnv/httpfiles/storage"
	"github.com/pkg/errors"
)

type storageFileWriter struct {
	basePath string
	size     int64

	file   *os.File
	writer io.Writer

	hash hash.Hash
}

func NewObjectWriter(basePath string, hash2 hash.Hash) (storage.ObjectWriter, error) {
	if err := os.MkdirAll(path.Join(basePath, "temp"), os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "ensure temp dir")
	}

	f, err := ioutil.TempFile(path.Join(basePath, "temp"), "")
	if err != nil {
		return nil, err
	}

	return &storageFileWriter{
		file:     f,
		hash:     hash2,
		writer:   io.MultiWriter(f, hash2),
		basePath: basePath,
	}, nil
}

func (w *storageFileWriter) Write(p []byte) (n int, err error) {
	w.size += int64(len(p))
	return w.writer.Write(p)
}

func (w *storageFileWriter) Save() (string, error) {
	if err := w.file.Close(); err != nil {
		return "", errors.Wrap(err, "close file")
	}
	hashSum := fmt.Sprintf("%x", w.hash.Sum(nil))

	if err := os.MkdirAll(path.Join(w.basePath, hashSum[:2]), os.ModePerm); err != nil {
		return "", errors.Wrap(err, "ensure destination folder")
	}

	if err := os.Rename(w.file.Name(), path.Join(w.basePath, hashSum[:2], hashSum)); err != nil {
		return "", errors.Wrap(err, "rename file")
	}

	return hashSum, nil
}

func (w *storageFileWriter) Size() int64 {
	return w.size
}

func (w *storageFileWriter) Remove() error {
	w.file.Close()
	return os.Remove(w.file.Name())
}
