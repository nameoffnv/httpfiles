package storage

import (
	"errors"
	"io"
)

var (
	ErrNotFound = errors.New("file not found")
)

type Storage interface {
	Get(string) (io.ReadCloser, error)
	Save(string, io.Reader) (int64, error)
	Delete(string) error
}
