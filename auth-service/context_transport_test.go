package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKernelTransportPreservesOperationAndParentCancellation(t *testing.T) {
	for _, parentCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "operation", true: "kernel"}[parentCancel], func(t *testing.T) {
			entered := make(chan struct{})
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
			defer s.Close()
			parent, stopParent := context.WithCancel(context.Background())
			defer stopParent()
			operation, stopOperation := context.WithCancel(context.Background())
			defer stopOperation()
			req, _ := http.NewRequestWithContext(operation, "GET", s.URL, nil)
			done := make(chan error, 1)
			go func() { _, err := (contextTransport{parent}).RoundTrip(req); done <- err }()
			<-entered
			if parentCancel {
				stopParent()
			} else {
				stopOperation()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancellation ignored")
				}
			case <-time.After(time.Second):
				t.Fatal("request stuck")
			}
		})
	}
}
func TestKernelTransportBodySurvivesHeadersAndCanBeRead(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer s.Close()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequest("GET", s.URL, nil)
	resp, err := (contextTransport{parent}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != "ok" {
		t.Fatal(string(data), err)
	}
}
