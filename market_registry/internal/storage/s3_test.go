package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestS3StorageReadWriteExistsListDelete(t *testing.T) {
	server := newFakeS3Server(t)
	defer server.Close()

	store, err := NewS3Storage(Config{
		Type:              "s3",
		BaseURL:           "https://registry.example.com",
		S3Endpoint:        server.URL,
		S3Region:          "auto",
		S3Bucket:          "market-artifacts",
		S3Prefix:          "registry/prod",
		S3AccessKeyID:     "ak",
		S3SecretAccessKey: "sk",
		S3UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("NewS3Storage returned error: %v", err)
	}

	ctx := context.Background()
	if err := store.Write(ctx, "index/catalog.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Write catalog returned error: %v", err)
	}
	if err := store.Write(ctx, "artifacts/plugin_package/demo/1.0.0/manifest.json", []byte(`{"version":"1.0.0"}`)); err != nil {
		t.Fatalf("Write manifest returned error: %v", err)
	}

	exists, err := store.Exists(ctx, "index/catalog.json")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected catalog object to exist")
	}

	body, err := store.Read(ctx, "index/catalog.json")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected read payload %q", string(body))
	}

	single, err := store.List(ctx, "index/catalog.json")
	if err != nil {
		t.Fatalf("List single file returned error: %v", err)
	}
	if len(single) != 1 || single[0] != "index/catalog.json" {
		t.Fatalf("unexpected single file list %#v", single)
	}

	allFiles, err := store.List(ctx, "artifacts")
	if err != nil {
		t.Fatalf("List artifacts returned error: %v", err)
	}
	if len(allFiles) != 1 || allFiles[0] != "artifacts/plugin_package/demo/1.0.0/manifest.json" {
		t.Fatalf("unexpected artifacts list %#v", allFiles)
	}

	if got := store.PublicURL("v1/catalog"); got != "https://registry.example.com/v1/catalog" {
		t.Fatalf("unexpected public url %q", got)
	}

	if err := store.Delete(ctx, "index/catalog.json"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	exists, err = store.Exists(ctx, "index/catalog.json")
	if err != nil {
		t.Fatalf("Exists after delete returned error: %v", err)
	}
	if exists {
		t.Fatal("expected deleted catalog object to disappear")
	}
}

func TestS3StorageRejectsTraversal(t *testing.T) {
	server := newFakeS3Server(t)
	defer server.Close()

	store, err := NewS3Storage(Config{
		Type:              "s3",
		BaseURL:           "https://registry.example.com",
		S3Endpoint:        server.URL,
		S3Bucket:          "market-artifacts",
		S3AccessKeyID:     "ak",
		S3SecretAccessKey: "sk",
		S3UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("NewS3Storage returned error: %v", err)
	}

	if err := store.Write(context.Background(), "../escape.json", []byte("{}")); err == nil {
		t.Fatal("expected traversal write to be rejected")
	}
}

type fakeS3Server struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeS3Server(t *testing.T) *httptest.Server {
	t.Helper()
	state := &fakeS3Server{
		objects: make(map[string][]byte),
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.handle(w, r)
	}))
}

func (s *fakeS3Server) handle(w http.ResponseWriter, r *http.Request) {
	bucket, key := parseFakeS3Path(r.URL.Path)
	if bucket == "" {
		http.NotFound(w, r)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		s.handleList(w, r, bucket)
		return
	case r.Method == http.MethodPut:
		payload, _ := io.ReadAll(r.Body)
		payload = decodeAWSChunkedPayload(payload)
		s.mu.Lock()
		s.objects[bucket+"/"+key] = append([]byte(nil), payload...)
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	case r.Method == http.MethodHead:
		s.mu.Lock()
		payload, ok := s.objects[bucket+"/"+key]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", "\"test-etag\"")
		w.WriteHeader(http.StatusOK)
		return
	case r.Method == http.MethodGet:
		s.mu.Lock()
		payload, ok := s.objects[bucket+"/"+key]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", "\"test-etag\"")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	case r.Method == http.MethodDelete:
		s.mu.Lock()
		delete(s.objects, bucket+"/"+key)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *fakeS3Server) handleList(w http.ResponseWriter, r *http.Request, bucket string) {
	prefix := strings.TrimLeft(r.URL.Query().Get("prefix"), "/")
	s.mu.Lock()
	keys := make([]string, 0, len(s.objects))
	for fullKey := range s.objects {
		if !strings.HasPrefix(fullKey, bucket+"/") {
			continue
		}
		key := strings.TrimPrefix(fullKey, bucket+"/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		keys = append(keys, key)
	}
	s.mu.Unlock()
	sort.Strings(keys)

	type content struct {
		Key  string `xml:"Key"`
		Size int    `xml:"Size"`
	}
	type listBucketResult struct {
		XMLName     xml.Name  `xml:"ListBucketResult"`
		Name        string    `xml:"Name"`
		Prefix      string    `xml:"Prefix"`
		KeyCount    int       `xml:"KeyCount"`
		MaxKeys     int       `xml:"MaxKeys"`
		IsTruncated bool      `xml:"IsTruncated"`
		Contents    []content `xml:"Contents"`
	}

	result := listBucketResult{
		Name:        bucket,
		Prefix:      prefix,
		KeyCount:    len(keys),
		MaxKeys:     len(keys),
		IsTruncated: false,
		Contents:    make([]content, 0, len(keys)),
	}
	s.mu.Lock()
	for _, key := range keys {
		result.Contents = append(result.Contents, content{
			Key:  key,
			Size: len(s.objects[bucket+"/"+key]),
		})
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_ = xml.NewEncoder(w).Encode(result)
}

func parseFakeS3Path(raw string) (string, string) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", ""
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func decodeAWSChunkedPayload(payload []byte) []byte {
	if !bytes.Contains(payload, []byte("chunk-signature=")) {
		return payload
	}
	reader := bufio.NewReader(bytes.NewReader(payload))
	var out bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return payload
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sizeToken := strings.SplitN(line, ";", 2)[0]
		size, err := strconv.ParseInt(strings.TrimSpace(sizeToken), 16, 64)
		if err != nil {
			return payload
		}
		if size == 0 {
			return out.Bytes()
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(reader, chunk); err != nil {
			return payload
		}
		out.Write(chunk)
		if _, err := reader.ReadString('\n'); err != nil {
			return payload
		}
	}
}
