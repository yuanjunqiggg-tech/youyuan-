package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB

const bucketName = "checkpoints"

// 节流控制：SaveCheckpoint 在 flushInterval 内只执行一次 BoltDB 写入，
// 避免大规模导入时频繁的 db.Update() 导致锁竞争和磁盘 I/O 激增。
const flushInterval = 2 * time.Second

var (
	throttleMu      sync.Mutex
	lastFlushTime   time.Time
	pendingThrottle = map[string]*Checkpoint{}
	flushTimer      *time.Timer
)

func InitCheckpointDB(dataDir string) error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if db != nil {
		return fmt.Errorf("checkpoint database is already open")
	}
	opened, err := bolt.Open(dataDir+"/td_state.db", 0644, nil)
	if err != nil {
		return fmt.Errorf("InitCheckpointDB: %w", err)
	}
	err = opened.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		interrupted := map[string][]byte{}
		if err = b.ForEach(func(k, v []byte) error {
			var cp Checkpoint
			if json.Unmarshal(v, &cp) != nil || cp.Status != "running" {
				return nil
			}
			cp.Status = "paused"
			cp.Message = "执行端重启，已保留断点；可继续运行。" + cp.Message
			raw, e := json.Marshal(cp)
			if e != nil {
				return e
			}
			interrupted[string(k)] = raw
			return nil
		}); err != nil {
			return err
		}
		for key, raw := range interrupted {
			if err = b.Put([]byte(key), raw); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = opened.Close()
		return err
	}
	db = opened
	pendingThrottle = map[string]*Checkpoint{}
	lastFlushTime = time.Time{}
	return nil
}

func CloseDB() error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	flushErr := flushPendingLocked()
	if db == nil {
		return flushErr
	}
	closeErr := db.Close()
	db = nil
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

type Checkpoint struct {
	TaskID     string          `json:"task_id"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	Progress   float64         `json:"progress"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`        // 恢复位置
	Name       string          `json:"name"`        // 显示名称
	Params     json.RawMessage `json:"params"`      // 原始任务参数(路径/坐标/速度等)
	ServerCode string          `json:"server_code"` // 绑定的服务器号
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
}

// SaveCheckpoint 保存断点，带节流控制。
// 终端状态（done/failed/cancelled）立即写入；进度更新每 flushInterval 合并写入一次。
// 调用方无需感知节流逻辑。
func SaveCheckpoint(cp *Checkpoint) error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if db == nil {
		return fmt.Errorf("checkpoint database is not open")
	}
	if cp == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	// Serialize draining progress and final state with timer flushes. Otherwise
	// a previously drained progress snapshot can overwrite an already saved done.
	if cp.Status == "done" || cp.Status == "failed" || cp.Status == "cancelled" || cp.Status == "paused" {
		delete(pendingThrottle, cp.TaskID)
		if err := flushPendingLocked(); err != nil {
			return err
		}
		return saveCheckpointLocked(cp)
	}
	snapshot := *cp
	snapshot.Data = append(json.RawMessage(nil), cp.Data...)
	snapshot.Params = append(json.RawMessage(nil), cp.Params...)
	pendingThrottle[cp.TaskID] = &snapshot
	if time.Since(lastFlushTime) >= flushInterval {
		return flushPendingLocked()
	}
	if flushTimer == nil {
		flushTimer = time.AfterFunc(flushInterval, func() { _ = flushPending() })
	}
	return nil
}

func SaveCheckpointNow(cp *Checkpoint) error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if cp == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	delete(pendingThrottle, cp.TaskID)
	return saveCheckpointLocked(cp)
}

func FlushCheckpoints() { _ = flushPending() }

// Caller holds throttleMu, including across BoltDB writes and close/reopen.
func saveCheckpointLocked(cp *Checkpoint) error {
	if db == nil {
		return fmt.Errorf("checkpoint database is not open")
	}
	snapshot := *cp
	snapshot.UpdatedAt = time.Now().Unix()
	data, err := json.Marshal(&snapshot)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte(bucketName)).Put([]byte(cp.TaskID), data) })
}

func flushPending() error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	return flushPendingLocked()
}

func flushPendingLocked() error {
	if flushTimer != nil {
		flushTimer.Stop()
		flushTimer = nil
	}
	for id, cp := range pendingThrottle {
		if err := saveCheckpointLocked(cp); err != nil {
			return err
		}
		delete(pendingThrottle, id)
	}
	lastFlushTime = time.Now()
	return nil
}

func LoadCheckpoint(taskID string) (*Checkpoint, error) {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if db == nil {
		return nil, fmt.Errorf("checkpoint database is not open")
	}
	var cp *Checkpoint
	err := db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket([]byte(bucketName)).Get([]byte(taskID))
		if data == nil {
			return fmt.Errorf("未找到断点：%s", taskID)
		}
		return json.Unmarshal(data, &cp)
	})
	return cp, err
}

func ListCheckpoints() ([]*Checkpoint, error) {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if db == nil {
		return nil, fmt.Errorf("checkpoint database is not open")
	}
	var result []*Checkpoint
	err := db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).ForEach(func(k, v []byte) error {
			var cp Checkpoint
			if err := json.Unmarshal(v, &cp); err != nil {
				return err
			}
			result = append(result, &cp)
			return nil
		})
	})
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].UpdatedAt == result[j].UpdatedAt {
			return result[i].CreatedAt > result[j].CreatedAt
		}
		return result[i].UpdatedAt > result[j].UpdatedAt
	})
	return result, err
}

func ListCheckpointsByServer(serverCode string) ([]*Checkpoint, error) {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	if db == nil {
		return nil, fmt.Errorf("checkpoint database is not open")
	}
	var result []*Checkpoint
	err := db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).ForEach(func(k, v []byte) error {
			var cp Checkpoint
			if err := json.Unmarshal(v, &cp); err != nil {
				return err
			}
			if serverCode == "" || cp.ServerCode == serverCode {
				result = append(result, &cp)
			}
			return nil
		})
	})
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].UpdatedAt == result[j].UpdatedAt {
			return result[i].CreatedAt > result[j].CreatedAt
		}
		return result[i].UpdatedAt > result[j].UpdatedAt
	})
	return result, err
}

func DeleteCheckpoint(taskID string) error {
	throttleMu.Lock()
	defer throttleMu.Unlock()
	delete(pendingThrottle, taskID)
	if db == nil {
		return fmt.Errorf("checkpoint database is not open")
	}
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Delete([]byte(taskID))
	})
}
