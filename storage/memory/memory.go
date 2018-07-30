package memory

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"

	"hash"

	"github.com/nameoffnv/httpfiles/storage"
)

type MemoryStorage struct {
	objects  map[string][]byte
	hashFunc func() hash.Hash
	lock     sync.RWMutex
}

func New(h func() hash.Hash) storage.Storage {
	return &MemoryStorage{
		objects:  make(map[string][]byte),
		hashFunc: h,
	}
}

func (s MemoryStorage) Objects() map[string][]byte {
	return s.objects
}

func (s *MemoryStorage) NewObjectWriter() (storage.ObjectWriter, error) {
	return &objectWriter{
		h: s.hashFunc(),
		postSave: func(h string, b []byte) {
			s.lock.Lock()
			defer s.lock.Unlock()

			s.objects[h] = b
		},
	}, nil
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

func (s *MemoryStorage) Delete(id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.objects[id]; !ok {
		return storage.ErrNotFound
	}

	delete(s.objects, id)

	return nil
}
