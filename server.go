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

	storage storage.Storage
	hashFn  HashFunc
	options Options
	limiter *limiter

	PreSave  func(storage.Storage, *http.Request) error
	PostSave func(storage.Storage, *http.Request, string) error
}

type counterResponseWriter struct {
	http.ResponseWriter
	bytesTransferred int64
}

func (c *counterResponseWriter) Write(b []byte) (int, error) {
	c.bytesTransferred += int64(len(b))
	return c.ResponseWriter.Write(b)
}

func New(s storage.Storage, hashFn HashFunc, opts Options) (*FilesHandler, error) {
	opts.Setup()

	fh := &FilesHandler{
		ServeMux: http.NewServeMux(),
		storage:  s,
		hashFn:   hashFn,
		options:  opts,
		limiter:  newLimiter(opts),
	}

	fh.Handle("/", fh.WithContext(http.HandlerFunc(fh.handle)))

	return fh, nil
}

func (s *FilesHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	crw := &counterResponseWriter{ResponseWriter: rw}
	remoteAddr := strings.Split(req.RemoteAddr, ":")[0]

	if s.limiter.Acquire(remoteAddr) {
		s.ServeMux.ServeHTTP(crw, req)
		s.limiter.Release(remoteAddr)
		s.limiter.Transferred(remoteAddr, crw.bytesTransferred)
	} else {
		log.Printf("remote: %s - too many requests", remoteAddr)
		http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
	}
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
		crw := rw.(*counterResponseWriter)
		log.Printf("%s - %s - %d bytes - %d nsec", req.Method, req.URL.RequestURI(), crw.bytesTransferred, stop.Nanoseconds())
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

	limitReader := io.LimitReader(req.Body, s.options.MaxFileSizeBytes)
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
