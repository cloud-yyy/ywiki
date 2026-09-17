package testutil

import (
	"io"
	"net/http"
	"sync"
)

// RequestLog records the requests a stub server received so tests can assert
// on the path, method, headers, and body the client actually sent.
type RequestLog struct {
	mu       sync.Mutex
	requests []RecordedRequest
}

// RecordedRequest is one captured request.
type RecordedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// Record captures a request. It is called from the stub handler.
func (l *RequestLog) Record(r *http.Request) {
	body := ""
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = string(data)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Header: r.Header.Clone(),
		Body:   body,
	})
}

// Len returns the number of recorded requests.
func (l *RequestLog) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.requests)
}

// At returns the recorded request at index i.
func (l *RequestLog) At(i int) RecordedRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.requests[i]
}

// Last returns the most recently recorded request.
func (l *RequestLog) Last() RecordedRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.requests[len(l.requests)-1]
}
