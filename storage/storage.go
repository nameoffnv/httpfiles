package storage

import (
	"errors"
	"io"
)

var (
	ErrNotFound = errors.New("file not found")
)

type Storage interface {
	NewObjectWriter() (ObjectWriter, error)
	Get(string) (io.ReadCloser, error)
	Delete(string) error
}

type ObjectWriter interface {
	io.Writer
	Size() int64
	Save() (string, error)
	Remove() error
}
