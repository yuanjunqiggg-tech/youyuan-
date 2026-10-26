package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestResumableUploadLostAcknowledgementAndRestart(t *testing.T) {
	root := t.TempDir()
	mux := http.NewServeMux()
	yyUploadSessionRoutes(mux, root)
	call := func(method, path string, data []byte) (int, map[string]any) {
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var out map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e, w.Body.String())
		}
		return w.Code, out
	}
	id := strings.Repeat("a", 32)
	data := bytes.Repeat([]byte("命名与附魔结构-byte-content"), 60000)
	begin, _ := json.Marshal(map[string]any{"id": id, "name": "附魔箱.mcstructure", "size": len(data)})
	if code, _ := call("POST", "/api/youyuan/files/transfer", begin); code != 200 {
		t.Fatal(code)
	}
	// Simulate a response lost after begin: the same client ticket returns the same upload.
	if code, _ := call("POST", "/api/youyuan/files/transfer", begin); code != 200 {
		t.Fatal(code)
	}
	first := data[:yyUploadChunkSize]
	h := sha256.Sum256(first)
	path := "/api/youyuan/files/transfer?id=" + id + "&offset=0&sha256=" + hex.EncodeToString(h[:])
	if code, _ := call("PUT", path, first); code != 200 {
		t.Fatal(code)
	}
	// A lost PUT response may be retried, but must never append the chunk twice.
	if code, _ := call("PUT", path, first); code != 200 {
		t.Fatal(code)
	}
	// A restarted execution process reads durable bytes and metadata, not an in-memory session.
	mux = http.NewServeMux()
	yyUploadSessionRoutes(mux, root)
	for offset := yyUploadChunkSize; offset < len(data); {
		end := offset + yyUploadChunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		h := sha256.Sum256(chunk)
		if code, _ := call("PUT", "/api/youyuan/files/transfer?id="+id+"&offset="+strconv.Itoa(offset)+"&sha256="+hex.EncodeToString(h[:]), chunk); code != 200 {
			t.Fatal(code)
		}
		offset = end
	}
	full := sha256.Sum256(data)
	finish := "/api/youyuan/files/transfer?id=" + id + "&sha256=" + hex.EncodeToString(full[:])
	code, out := call("POST", finish, nil)
	if code != 200 {
		t.Fatal(out)
	}
	saved, e := os.ReadFile(out["path"].(string))
	if e != nil || !bytes.Equal(saved, data) {
		t.Fatal("final bytes differ", e)
	}
	code, retry := call("POST", finish, nil)
	if code != 200 || retry["path"] != out["path"] {
		t.Fatal("duplicate finalization", retry)
	}
	files, _ := os.ReadDir(filepath.Join(root, "用户上传"))
	if len(files) != 1 {
		t.Fatal("unexpected duplicate file count", len(files))
	}
	// The ticket cancellation endpoint never deletes an already committed user file.
	if code, _ := call("DELETE", "/api/youyuan/files/transfer?id="+id, nil); code != 200 {
		t.Fatal(code)
	}
	if _, e = os.Stat(out["path"].(string)); e != nil {
		t.Fatal(e)
	}
}

func TestUploadRejectsCorruptionAndUncommittedTail(t *testing.T) {
	dir := t.TempDir()
	s := &yyUploadStore{root: filepath.Join(dir, "sessions"), destination: filepath.Join(dir, "files")}
	for _, name := range []string{"../outside", "x/y", "x\\y", "", ".."} {
		if _, e := s.begin(name, 1, ""); e == nil {
			t.Fatal("accepted filename", name)
		}
	}
	ticket, e := s.begin("head.bdx", 4, "")
	if e != nil {
		t.Fatal(e)
	}
	data := []byte("test")
	h := sha256.Sum256(data)
	digest := hex.EncodeToString(h[:])
	if e = s.chunk(ticket, 0, data, strings.Repeat("0", 64)); e == nil {
		t.Fatal("accepted corrupt bytes")
	}
	if e = s.chunk(ticket, 1, data, digest); e == nil {
		t.Fatal("accepted out-of-order chunk")
	}
	// A crash between byte sync and metadata sync leaves a tail, which is rolled back.
	f, e := os.OpenFile(filepath.Join(s.root, ticket.ID+".part"), os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = io.Copy(f, bytes.NewReader(data))
	_ = f.Close()
	restored, e := s.load(ticket.ID)
	if e != nil {
		t.Fatal(e)
	}
	if restored.Offset != 0 {
		t.Fatal(restored.Offset)
	}
	info, _ := os.Stat(filepath.Join(s.root, ticket.ID+".part"))
	if info.Size() != 0 {
		t.Fatal("uncommitted tail retained")
	}
	if e = s.chunk(restored, 0, data, digest); e != nil {
		t.Fatal(e)
	}
	if e = s.finish(restored, strings.Repeat("0", 64)); e == nil {
		t.Fatal("accepted mismatched final digest")
	}
	if e = s.finish(restored, digest); e != nil {
		t.Fatal(e)
	}
	// Recover a crash after rename, when the preceding durable metadata says incomplete.
	restored.Complete = false
	restored.Path = ""
	restored.SHA256 = ""
	if e = s.persist(restored); e != nil {
		t.Fatal(e)
	}
	recovered, e := s.load(restored.ID)
	if e != nil || !recovered.Complete || recovered.SHA256 != digest {
		t.Fatal(recovered, e)
	}
}

func TestUploadNoOverwriteAndMissingCompletedFile(t *testing.T) {
	dir := t.TempDir()
	s := &yyUploadStore{root: filepath.Join(dir, "sessions"), destination: filepath.Join(dir, "files")}
	var paths []string
	for _, id := range []string{strings.Repeat("a", 32), strings.Repeat("a", 8) + strings.Repeat("b", 24)} {
		ticket, e := s.begin("same.bdx", 4, id)
		if e != nil {
			t.Fatal(e)
		}
		data := []byte(id[:1] + id[len(id)-1:] + "xy")
		h := sha256.Sum256(data)
		digest := hex.EncodeToString(h[:])
		if e = s.chunk(ticket, 0, data, digest); e != nil {
			t.Fatal(e)
		}
		if e = s.finish(ticket, digest); e != nil {
			t.Fatal(e)
		}
		paths = append(paths, ticket.Path)
	}
	if paths[0] == paths[1] {
		t.Fatal("colliding final paths")
	}
	first, _ := os.ReadFile(paths[0])
	if string(first) != "aaxy" {
		t.Fatal("previous upload overwritten")
	}
	if e := os.Remove(paths[0]); e != nil {
		t.Fatal(e)
	}
	if _, e := s.load(strings.Repeat("a", 32)); e == nil {
		t.Fatal("missing completed file falsely reported usable")
	}
	if e := os.WriteFile(paths[1], []byte("xxxx"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.load(strings.Repeat("a", 8) + strings.Repeat("b", 24)); e == nil {
		t.Fatal("changed completed file falsely reported usable")
	}
}

func TestSlowUploadBodyDoesNotBlockStatus(t *testing.T) {
	mux := http.NewServeMux()
	yyUploadSessionRoutes(mux, t.TempDir())
	id := strings.Repeat("d", 32)
	req, _ := json.Marshal(map[string]any{"id": id, "name": "slow.bdx", "size": 4})
	create := httptest.NewRecorder()
	mux.ServeHTTP(create, httptest.NewRequest("POST", "/api/youyuan/files/transfer", bytes.NewReader(req)))
	if create.Code != 200 {
		t.Fatal(create.Body.String())
	}
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", "/api/youyuan/files/transfer?id="+id+"&offset=0", reader))
	}()
	status := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/files/transfer?id="+id, nil))
		status <- w.Code
	}()
	select {
	case code := <-status:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("slow body blocked upload status")
	}
	_ = writer.CloseWithError(io.ErrUnexpectedEOF)
	<-done
}

func TestUploadRejectsExpiryWhileTicketIsBeingResumed(t *testing.T) {
	dir := t.TempDir()
	s := &yyUploadStore{root: filepath.Join(dir, "sessions"), destination: filepath.Join(dir, "files")}
	old, e := s.begin("resuming.bdx", 4, "")
	if e != nil {
		t.Fatal(e)
	}
	meta := filepath.Join(s.root, old.ID+".json")
	past := time.Now().Add(-25 * time.Hour)
	if e = os.Chtimes(meta, past, past); e != nil {
		t.Fatal(e)
	}
	guard := s.ticketLock(old.ID)
	guard.Lock()
	_, e = s.begin("another.bdx", 4, "")
	guard.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(meta); e != nil {
		t.Fatal("active ticket expired while owned", e)
	}
	if _, e = s.begin("third.bdx", 4, ""); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(meta); !os.IsNotExist(e) {
		t.Fatal("expired unowned ticket was not collected", e)
	}
}
