package memory

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"

	"github.com/nameoffnv/httpfiles/storage"
	"github.com/pkg/errors"
)

type MemoryStorage struct {
	objects map[string][]byte
	lock    sync.RWMutex
}

func New() storage.Storage {
	return &MemoryStorage{
		objects: make(map[string][]byte),
	}
}

func (s MemoryStorage) Objects() map[string][]byte {
	return s.objects
}

func (s *MemoryStorage) Get(id string) (io.ReadCloser, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	b, ok := s.objects[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return ioutil.NopCloser(bytes.NewReader(b)), nil
}

func (s *MemoryStorage) Save(h string, data io.Reader) (int64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	b, err := ioutil.ReadAll(data)
	if err != nil {
		return 0, errors.Wrap(err, "read all")
	}

	s.objects[h] = b

	return int64(len(b)), nil
}

func (s *MemoryStorage) Delete(id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.objects[id]; !ok {
		return storage.ErrNotFound
	}

	delete(s.objects, id)

	return nil
}
