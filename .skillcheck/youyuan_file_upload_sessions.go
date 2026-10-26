package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transfer tickets live inside the selected execution device's authenticated API.
// Only byte writes are idempotent here; game commands are never replayed.
const yyUploadChunkSize = 1 << 20
const yyUploadFileLimit int64 = 2 << 30

type yyUploadChunk struct {
	Offset int64  `json:"offset"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}
type yyUploadTicket struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Size     int64           `json:"size"`
	Offset   int64           `json:"offset"`
	Chunks   []yyUploadChunk `json:"chunks"`
	Complete bool            `json:"complete"`
	Path     string          `json:"path,omitempty"`
	SHA256   string          `json:"sha256,omitempty"`
	Updated  time.Time       `json:"updated"`
}
type yyUploadStore struct {
	mu                sync.Mutex
	tickets           [64]sync.Mutex
	root, destination string
}

func (s *yyUploadStore) ticketLock(id string) *sync.Mutex {
	sum := sha256.Sum256([]byte(id))
	return &s.tickets[int(sum[0])%len(s.tickets)]
}

var yyUploadID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var yyUploadHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *yyUploadStore) persist(t *yyUploadTicket) error {
	t.Updated = time.Now().UTC()
	data, e := json.Marshal(t)
	if e != nil {
		return e
	}
	p := filepath.Join(s.root, t.ID+".json")
	f, e := os.OpenFile(p+".new", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(p+".new", p)
}
func (s *yyUploadStore) load(id string) (*yyUploadTicket, error) {
	if !yyUploadID.MatchString(id) {
		return nil, errors.New("无效上传编号")
	}
	raw, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if e != nil {
		return nil, e
	}
	var t yyUploadTicket
	if e = json.Unmarshal(raw, &t); e != nil {
		return nil, e
	}
	if t.ID != id || !yyUploadFilename(t.Name) || t.Size < 0 || t.Size > yyUploadFileLimit || t.Offset < 0 || t.Offset > t.Size {
		return nil, errors.New("上传记录损坏")
	}
	if t.Complete {
		path := s.finalPath(&t)
		info, err := os.Stat(path)
		if err != nil || info.Size() != t.Size {
			return nil, errors.New("已上传文件不存在或大小已变化，请重新上传")
		}
		digest, err := yyUploadFileSHA(path)
		if err != nil || digest != t.SHA256 {
			return nil, errors.New("已上传文件内容已变化，请重新上传")
		}
		t.Path = path
	}
	if !t.Complete {
		part := filepath.Join(s.root, id+".part")
		if info, e := os.Stat(part); e == nil {
			if info.Size() < t.Offset {
				return nil, errors.New("上传内容短于已确认位置")
			}
			if info.Size() > t.Offset {
				if e = os.Truncate(part, t.Offset); e != nil {
					return nil, e
				}
			}
		} else if os.IsNotExist(e) && t.Offset == t.Size {
			// Recover a crash after the atomic rename but before metadata commit.
			path := s.finalPath(&t)
			if info, e := os.Stat(path); e == nil && info.Size() == t.Size {
				t.Path = path
				t.Complete = true
				t.SHA256, e = yyVerifyUploadChunks(path, &t)
				if e != nil {
					return nil, e
				}
				if e = s.persist(&t); e != nil {
					return nil, e
				}
			} else {
				return nil, errors.New("上传暂存文件不存在")
			}
		} else {
			return nil, e
		}
	}
	return &t, nil
}
func yyUploadFilename(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 180 && !strings.ContainsAny(name, "/\\\x00")
}
func (s *yyUploadStore) finalPath(t *yyUploadTicket) string {
	ext := filepath.Ext(t.Name)
	base := strings.TrimSuffix(t.Name, ext)
	return filepath.Join(s.destination, base+" ("+t.ID+")"+ext)
}
func yyUploadFileSHA(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func yyVerifyUploadChunks(path string, t *yyUploadTicket) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	whole := sha256.New()
	var offset int64
	for _, chunk := range t.Chunks {
		if chunk.Offset != offset || chunk.Size < 1 || chunk.Size > yyUploadChunkSize {
			return "", errors.New("分块记录损坏")
		}
		part := sha256.New()
		n, e := io.CopyN(io.MultiWriter(whole, part), f, int64(chunk.Size))
		if e != nil {
			return "", e
		}
		if hex.EncodeToString(part.Sum(nil)) != chunk.SHA256 {
			return "", errors.New("已存储分块与客户端SHA-256不匹配")
		}
		offset += n
	}
	var extra [1]byte
	n, e := f.Read(extra[:])
	if offset != t.Size || n != 0 || e != io.EOF {
		return "", errors.New("完整文件大小与分块记录不一致")
	}
	return hex.EncodeToString(whole.Sum(nil)), nil
}
func (s *yyUploadStore) begin(name string, size int64, id string) (*yyUploadTicket, error) {
	if !yyUploadFilename(name) || size < 0 || size > yyUploadFileLimit {
		return nil, errors.New("文件名无效或超过2GiB单文件上限")
	}
	if id != "" {
		if !yyUploadID.MatchString(id) {
			return nil, errors.New("无效上传编号")
		}
		if prior, e := s.load(id); e == nil {
			if prior.Name != name || prior.Size != size {
				return nil, errors.New("上传编号已用于其他文件")
			}
			return prior, nil
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	if e := os.MkdirAll(s.root, 0700); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	active := 0
	var reserved int64
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !yyUploadID.MatchString(id) {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			continue
		}
		if time.Since(info.ModTime()) > 24*time.Hour {
			// A resumed writer/finalizer owns the same stripe. Never delete its
			// bytes based on the directory snapshot taken before it resumed.
			guard := s.ticketLock(id)
			if guard.TryLock() {
				meta := filepath.Join(s.root, id+".json")
				fresh, err := os.Stat(meta)
				removed := false
				if err == nil && time.Since(fresh.ModTime()) > 24*time.Hour {
					err = os.Remove(filepath.Join(s.root, id+".part"))
					if err == nil || os.IsNotExist(err) {
						removed = os.Remove(meta) == nil
					}
				}
				guard.Unlock()
				if removed {
					continue
				}
			}
		}
		if raw, e := os.ReadFile(filepath.Join(s.root, entry.Name())); e == nil {
			var t yyUploadTicket
			if json.Unmarshal(raw, &t) == nil && !t.Complete {
				active++
				reserved += t.Size
			}
		}
	}
	if active >= 8 || reserved+size > 4<<30 {
		return nil, errors.New("未完成上传过多，请先续传已有文件；暂存记录保留24小时")
	}
	if id == "" {
		var b [16]byte
		if _, e = rand.Read(b[:]); e != nil {
			return nil, e
		}
		id = hex.EncodeToString(b[:])
	}
	f, e := os.OpenFile(filepath.Join(s.root, id+".part"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	t := &yyUploadTicket{ID: id, Name: name, Size: size, Chunks: []yyUploadChunk{}}
	if e = s.persist(t); e != nil {
		_ = os.Remove(filepath.Join(s.root, id+".part"))
		return nil, e
	}
	return t, nil
}
func (s *yyUploadStore) chunk(t *yyUploadTicket, offset int64, data []byte, digest string) error {
	if len(data) == 0 || len(data) > yyUploadChunkSize || offset < 0 || int64(len(data))+offset > t.Size || !yyUploadHash.MatchString(digest) {
		return errors.New("分块范围或校验值无效")
	}
	remaining := t.Size - offset
	expected := int64(yyUploadChunkSize)
	if remaining < expected {
		expected = remaining
	}
	if int64(len(data)) != expected {
		return errors.New("分块必须为1MiB，最后一块除外")
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != digest {
		return errors.New("分块SHA-256校验失败")
	}
	if offset < t.Offset {
		for _, chunk := range t.Chunks {
			if chunk.Offset == offset && chunk.Size == len(data) && chunk.SHA256 == digest {
				return nil
			}
		}
		return errors.New("该位置已有不同内容，拒绝覆盖")
	}
	if t.Complete || offset != t.Offset {
		return fmt.Errorf("请从已确认位置 %d 继续", t.Offset)
	}
	f, e := os.OpenFile(filepath.Join(s.root, t.ID+".part"), os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.WriteAt(data, offset)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	t.Offset += int64(len(data))
	t.Chunks = append(t.Chunks, yyUploadChunk{Offset: offset, Size: len(data), SHA256: digest})
	return s.persist(t)
}
func (s *yyUploadStore) finish(t *yyUploadTicket, expected string) error {
	if expected != "" && !yyUploadHash.MatchString(expected) {
		return errors.New("无效文件SHA-256")
	}
	if t.Complete {
		if expected != "" && expected != t.SHA256 {
			return errors.New("文件SHA-256不匹配")
		}
		return nil
	}
	if t.Offset != t.Size {
		return errors.New("文件尚未上传完整")
	}
	part := filepath.Join(s.root, t.ID+".part")
	digest, e := yyVerifyUploadChunks(part, t)
	if e != nil {
		return e
	}
	if expected != "" && digest != expected {
		return errors.New("文件SHA-256不匹配")
	}
	if e = os.MkdirAll(s.destination, 0700); e != nil {
		return e
	}
	dst := s.finalPath(t)
	// Linking a fully synced temporary file commits atomically without replacing
	// any existing user file. Both directories are under the same data volume.
	if e = os.Link(part, dst); e != nil {
		if !os.IsExist(e) {
			return e
		}
		existing, err := yyUploadFileSHA(dst)
		if err != nil || existing != digest {
			return errors.New("目标文件已存在且内容不同，拒绝覆盖")
		}
	}
	if e = os.Remove(part); e != nil {
		return e
	}
	t.Path = dst
	t.SHA256 = digest
	t.Complete = true
	return s.persist(t)
}
func yyUploadSessionRoutes(mux *http.ServeMux, dataDir string) {
	s := &yyUploadStore{root: filepath.Join(dataDir, ".upload-sessions"), destination: filepath.Join(dataDir, "用户上传")}
	mux.HandleFunc("/api/youyuan/files/transfer", func(w http.ResponseWriter, r *http.Request) {
		fail := func(code int, e error) {
			w.WriteHeader(code)
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		}
		var t *yyUploadTicket
		var e error
		var chunkData []byte
		if r.Method == http.MethodPut {
			// An abandoned upload body must not hold any ticket or store lock.
			chunkData, e = io.ReadAll(io.LimitReader(r.Body, yyUploadChunkSize+1))
			if e != nil {
				fail(400, e)
				return
			}
			if len(chunkData) > yyUploadChunkSize {
				fail(413, errors.New("单个分块超过1MiB"))
				return
			}
		}
		if r.Method == http.MethodPost && r.URL.Query().Get("id") == "" {
			var req struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Size int64  `json:"size"`
			}
			if e = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); e != nil {
				fail(400, e)
				return
			}
			guard := s.ticketLock(req.ID)
			guard.Lock()
			defer guard.Unlock()
			s.mu.Lock()
			t, e = s.begin(req.Name, req.Size, req.ID)
			s.mu.Unlock()
		} else {
			guard := s.ticketLock(r.URL.Query().Get("id"))
			guard.Lock()
			defer guard.Unlock()
			t, e = s.load(r.URL.Query().Get("id"))
			if e != nil {
				fail(404, errors.New("上传记录不存在或已过期，请重新选择文件"))
				return
			}
			switch r.Method {
			case http.MethodGet:
			case http.MethodPut:
				offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
				if err != nil {
					fail(400, err)
					return
				}
				e = s.chunk(t, offset, chunkData, r.URL.Query().Get("sha256"))
			case http.MethodPost:
				e = s.finish(t, r.URL.Query().Get("sha256"))
			case http.MethodDelete:
				if !t.Complete {
					if e = os.Remove(filepath.Join(s.root, t.ID+".part")); e != nil && !os.IsNotExist(e) {
						fail(409, e)
						return
					}
				}
				if e = os.Remove(filepath.Join(s.root, t.ID+".json")); e != nil && !os.IsNotExist(e) {
					fail(409, e)
					return
				}
				writeJSON(w, map[string]any{"ok": true, "cancelled": !t.Complete})
				return
			default:
				w.WriteHeader(405)
				return
			}
		}
		if e != nil {
			fail(409, e)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "upload": t, "chunk_size": yyUploadChunkSize, "bytes": t.Size, "name": filepath.Base(t.Path), "path": t.Path})
	})
}
