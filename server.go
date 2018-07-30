package httpfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nameoffnv/httpfiles/storage"
)

type ctxKey int

const (
	ctxStorageKey ctxKey = iota
)

type FilesHandler struct {
	*http.ServeMux

	storage     storage.Storage
	hashFn      HashFunc
	maxFileSize int64

	PreSave  func(storage.Storage, *http.Request) error
	PostSave func(storage.Storage, *http.Request, string) error
}

func New(s storage.Storage, hashFn HashFunc, maxFileSize int64) (*FilesHandler, error) {
	fh := &FilesHandler{
		ServeMux:    http.NewServeMux(),
		storage:     s,
		hashFn:      hashFn,
		maxFileSize: maxFileSize,
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

	limitReader := io.LimitReader(req.Body, s.maxFileSize)
	bodyBytes, err := ioutil.ReadAll(limitReader)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// check url params for hashes
	checkHashes := make(map[string]string)
	for k, v := range req.URL.Query() {
		for _, vv := range v {
			if _, ok := HashFuncs[k]; ok {
				checkHashes[k] = vv
			}
		}
	}

	if err := ValidateHashes(bodyBytes, checkHashes); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	h, err := s.hashFn(bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := s.storage.Save(h, bytes.NewReader(bodyBytes)); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Body.Close()

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
