package httpfiles_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crypto/sha256"

	"github.com/nameoffnv/httpfiles"
	"github.com/nameoffnv/httpfiles/storage/memory"
)

func TestFilesHandler(t *testing.T) {
	s := memory.New(sha256.New)

	handler, err := httpfiles.New(s)
	if err != nil {
		t.Fatal(err)
	}

	testObj := []byte("hello world")

	t.Run("bad-method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/", bytes.NewReader(testObj))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusMethodNotAllowed, rr.Code)
		}
	})

	t.Run("upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(testObj))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("bad response status code, excepted %d, acutal %d", http.StatusCreated, rr.Code)
		}

		respMap := make(map[string]string)
		if err := json.NewDecoder(rr.Body).Decode(&respMap); err != nil {
			t.Fatalf("json decode response failed, error %v", err)
		}

		hashKey, ok := respMap["hash"]
		if !ok {
			t.Fatal("not found 'hash' field in response")
		}

		storageObjects := s.(*memory.MemoryStorage).Objects()
		obj, ok := storageObjects[hashKey]
		if !ok {
			t.Fatalf("not found object in memory storage with key %s", hashKey)
		}
		if string(obj) != string(testObj) {
			t.Fatalf("test object not equal object from memory, '%s' != '%s'", string(testObj), string(obj))
		}
	})

	t.Run("upload-bad-hash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/?md5=aaa", bytes.NewReader(testObj))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("bad response status code, excepted %d, acutal %d", http.StatusBadRequest, rr.Code)
		}

		if !strings.Contains(rr.Body.String(), "hash mismatch") {
			t.Fatalf("bad response body '%s' excepted string which contains 'hash mismatch'", strings.TrimSpace(rr.Body.String()))
		}
	})

	t.Run("upload-hash-validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/?sha256=b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", bytes.NewReader(testObj))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("bad response status code, excepted %d, acutal %d", http.StatusCreated, rr.Code)
		}
	})

	t.Run("get", func(t *testing.T) {
		var hashKey string

		storageObjects := s.(*memory.MemoryStorage).Objects()
		for k := range storageObjects {
			hashKey = k
			break
		}

		if hashKey == "" {
			t.Fatal("not found object in memory storage")
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprint("/", hashKey), nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusOK, rr.Code)
		}

		if string(testObj) != rr.Body.String() {
			t.Fatalf("test object not equal object from memory, '%s' != '%s'", string(testObj), rr.Body.String())
		}
	})

	t.Run("get-key-not-exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/notfoundkey", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("get-bad-url", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/some/bad/url", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var hashKey string

		storageObjects := s.(*memory.MemoryStorage).Objects()
		for k := range storageObjects {
			hashKey = k
			break
		}

		if hashKey == "" {
			t.Fatal("not found object in memory storage")
		}

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprint("/", hashKey), nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusNoContent, rr.Code)
		}

		if _, ok := storageObjects[hashKey]; ok {
			t.Fatalf("object exist in memory storage after deletion, key %s", hashKey)
		}
	})

	t.Run("delete-key-not-exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/badkey", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("bad response status code, excepted %d, actual %d", http.StatusNotFound, rr.Code)
		}
	})
}
