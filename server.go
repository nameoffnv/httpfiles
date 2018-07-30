package httpfiles

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"fmt"
	"hash"

	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"github.com/nameoffnv/httpfiles/storage"
)

type ctxKey int

const (
	ctxStorageKey ctxKey = iota
)

var Hashes = map[string]func() hash.Hash{
	"sha1":   sha1.New,
	"sha256": sha256.New,
	"md5":    md5.New,
}

type FilesHandler struct {
	*http.ServeMux

	storage     storage.Storage
	maxFileSize int64

	PreSave  func(storage.Storage, *http.Request) error
	PostSave func(storage.Storage, *http.Request, string) error
}

func New(s storage.Storage) (*FilesHandler, error) {
	fh := &FilesHandler{
		ServeMux: http.NewServeMux(),
		storage:  s,
	}

	fh.Handle("/", fh.WithContext(http.HandlerFunc(fh.handle)))

	return fh, nil
}

func (s *FilesHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.ServeMux.ServeHTTP(rw, req)
}

func (s *FilesHandler) GetStorage(req *http.Request) storage.Storage {
	cStorage, ok := req.Context().Value(ctxStorageKey).(storage.Storage)
	if !ok {
		return nil
	}
	return cStorage
}

func (s *FilesHandler) WithContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now()
		ctx := context.WithValue(req.Context(), ctxStorageKey, s.storage)
		next.ServeHTTP(rw, req.WithContext(ctx))
		stop := time.Now().Sub(start)
		log.Printf("%s - %s - %d nsec", req.Method, req.URL.RequestURI(), stop.Nanoseconds())
	})
}

func (s *FilesHandler) handle(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		s.handleGET(rw, req)
	case http.MethodPost:
		s.handlePOST(rw, req)
	case http.MethodDelete:
		s.handleDELETE(rw, req)
	default:
		http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (s *FilesHandler) handleGET(rw http.ResponseWriter, req *http.Request) {
	urlParts := strings.Split(req.URL.RequestURI(), "/")
	if len(urlParts) != 2 {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	reader, err := s.storage.Get(urlParts[1])
	if err == storage.ErrNotFound {
		http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	if _, err := io.Copy(rw, reader); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (s *FilesHandler) handlePOST(rw http.ResponseWriter, req *http.Request) {
	if s.PreSave != nil {
		if err := s.PreSave(s.storage, req); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	objectWriter, err := s.storage.NewObjectWriter()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	writers := []io.Writer{
		objectWriter,
	}

	// check url params for hashes
	checkHashes := make(map[string]hash.Hash)
	for k := range req.URL.Query() {
		if hNew, ok := Hashes[k]; ok {
			h := hNew()
			checkHashes[k] = h
			writers = append(writers, h)
		}
	}

	mw := io.MultiWriter(writers...)

	if _, err := io.Copy(mw, req.Body); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range checkHashes {
		hashProvided := req.URL.Query().Get(k)
		hashCalculated := fmt.Sprintf("%x", v.Sum(nil))
		if hashProvided != hashCalculated {
			objectWriter.Remove()
			http.Error(
				rw,
				fmt.Sprintf("hash mismatch %s, %s != %s", k, hashProvided, hashCalculated),
				http.StatusBadRequest,
			)
			return
		}
	}

	h, err := objectWriter.Save()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.PostSave != nil {
		if err := s.PostSave(s.storage, req, h); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	rw.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(rw).Encode(map[string]string{"hash": h}); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (s *FilesHandler) handleDELETE(rw http.ResponseWriter, req *http.Request) {
	urlParts := strings.Split(req.URL.RequestURI(), "/")
	if len(urlParts) != 2 {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := s.storage.Delete(urlParts[1]); err == storage.ErrNotFound {
		http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
