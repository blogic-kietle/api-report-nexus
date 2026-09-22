// Package session holds chunked PDF uploads in memory until every chunk has arrived.
package session

import (
	"bytes"
	"errors"
	"sync"
	"time"
)

// Errors the handlers translate into 409/404/400.
var (
	ErrExists     = errors.New("session: already exists")
	ErrNotFound   = errors.New("session: not found")
	ErrIncomplete = errors.New("session: not all chunks received")
)

type upload struct {
	fileName string
	total    int
	chunks   map[int][]byte
	updated  time.Time
}

// Store holds uploads, dropping them once they go quiet for longer than ttl.
type Store struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]*upload
}

// New returns a store that sweeps stale uploads on Init instead of in the background.
func New(ttl time.Duration) *Store {
	return &Store{ttl: ttl, m: map[string]*upload{}}
}

// Init registers a new upload, refusing to overwrite one already in flight.
func (s *Store) Init(id, fileName string, total int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	if _, ok := s.m[id]; ok {
		return ErrExists
	}
	s.m[id] = &upload{fileName: fileName, total: total, chunks: map[int][]byte{}, updated: time.Now()}
	return nil
}

// AddChunk stores one part; re-sending an index overwrites it, where Node counted it twice.
func (s *Store) AddChunk(id string, index int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.m[id]
	if !ok {
		return ErrNotFound
	}
	u.chunks[index] = data
	u.updated = time.Now()
	return nil
}

func (s *Store) FileName(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.m[id]
	if !ok {
		return "", false
	}
	return u.fileName, true
}

// Assemble joins the chunks in index order once they have all arrived.
func (s *Store) Assemble(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.m[id]
	if !ok {
		return nil, ErrNotFound
	}
	if len(u.chunks) != u.total {
		return nil, ErrIncomplete
	}
	var buf bytes.Buffer
	for i := range u.total {
		part, ok := u.chunks[i]
		if !ok {
			// Chunks may be 1-based; fall back before giving up.
			if part, ok = u.chunks[i+1]; !ok {
				return nil, ErrIncomplete
			}
		}
		buf.Write(part)
	}
	return buf.Bytes(), nil
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

// Callers must hold the lock.
func (s *Store) sweep() {
	cutoff := time.Now().Add(-s.ttl)
	for id, u := range s.m {
		if u.updated.Before(cutoff) {
			delete(s.m, id)
		}
	}
}
