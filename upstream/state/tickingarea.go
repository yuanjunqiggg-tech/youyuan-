package state

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

type TickingAreaLease struct {
	Server    string `json:"server"`
	Dimension string `json:"dimension"`
	Name      string `json:"name"`
}

const tickingLeaseBucket = "tickingarea_leases"

// Persist before sending add: a timed-out command may still have created the area.
func RememberTickingArea(v TickingAreaLease) error {
	if db == nil {
		return fmt.Errorf("常加载区域归属记录库未就绪")
	}
	if v.Server == "" || v.Name == "" || v.Dimension == "" {
		return fmt.Errorf("常加载区域归属记录不完整")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(tickingLeaseBucket))
		if err != nil {
			return err
		}
		key := []byte(v.Server + "\x00" + v.Name)
		if b.Get(key) == nil && b.Stats().KeyN >= 1000 {
			return fmt.Errorf("遗留常加载区域记录过多，请先恢复连接并清理")
		}
		return b.Put(key, raw)
	})
}

func ForgetTickingArea(v TickingAreaLease) error {
	if db == nil {
		return fmt.Errorf("常加载区域归属记录库未就绪")
	}
	return db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(tickingLeaseBucket)); b != nil {
			return b.Delete([]byte(v.Server + "\x00" + v.Name))
		}
		return nil
	})
}

func ListTickingAreas(server string) (result []TickingAreaLease, err error) {
	if db == nil {
		return nil, fmt.Errorf("常加载区域归属记录库未就绪")
	}
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tickingLeaseBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, raw []byte) error {
			var v TickingAreaLease
			if err := json.Unmarshal(raw, &v); err != nil {
				return err
			}
			if v.Server == server {
				result = append(result, v)
			}
			return nil
		})
	})
	return
}
