package playerrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type delivery struct {
	cfg      Config
	sequence uint64
	client   *http.Client
	lock     *os.File
}

func newDelivery(c Config) (*delivery, error) {
	if e := os.MkdirAll(filepath.Dir(c.SequenceFile), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(c.SequenceFile+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, fmt.Errorf("采集序号文件被占用；确认旧代理已退出后才能清理 lock")
	}
	d := &delivery{cfg: c, lock: f, client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	b, e := os.ReadFile(c.SequenceFile)
	if e != nil && !os.IsNotExist(e) {
		d.Close()
		return nil, e
	}
	if len(b) > 0 && json.Unmarshal(b, &d.sequence) != nil {
		d.Close()
		return nil, fmt.Errorf("采集序号文件损坏；不能从零重用序号")
	}
	return d, nil
}
func (d *delivery) Close() {
	if d.lock != nil {
		_ = d.lock.Close()
		_ = os.Remove(d.cfg.SequenceFile + ".lock")
		d.lock = nil
	}
}
func (d *delivery) send(ctx context.Context, events []Event) (int, error) {
	if d.sequence >= (1<<53)-1 {
		return 0, fmt.Errorf("采集序号耗尽，请创建新来源")
	}
	next := d.sequence + 1
	b, _ := json.Marshal(next)
	tmp := d.cfg.SequenceFile + ".tmp"
	// Reserve before HTTP. A crash can skip a sequence but cannot duplicate one.
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return 0, fmt.Errorf("序号持久化失败")
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(tmp, d.cfg.SequenceFile)
	}
	if e != nil {
		return 0, fmt.Errorf("序号持久化失败")
	}
	d.sequence = next
	body, _ := json.Marshal(map[string]any{"source_id": d.cfg.SourceID, "sequence": next, "events": events})
	for attempt := 0; attempt < 3; attempt++ {
		r, e := http.NewRequestWithContext(ctx, "POST", d.cfg.BridgeURL, bytes.NewReader(body))
		if e != nil {
			return 0, fmt.Errorf("采集桥地址无效")
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Youyuan-Observer-Token", d.cfg.BridgeToken)
		for k, v := range d.cfg.BridgeHeaders {
			r.Header.Set(k, v)
		}
		resp, e := d.client.Do(r)
		if e == nil {
			var reply struct {
				OK     bool `json:"ok"`
				Result struct {
					Accepted  int  `json:"accepted"`
					Duplicate bool `json:"duplicate"`
				} `json:"result"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 && decodeErr == nil && reply.OK {
				if reply.Result.Duplicate {
					return len(events), nil
				} // Retried receipt of this same sequence.
				if reply.Result.Accepted == len(events) {
					return reply.Result.Accepted, nil
				}
				return 0, fmt.Errorf("采集桥返回不完整接收数")
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return 0, fmt.Errorf("采集桥拒绝（HTTP %d）", resp.StatusCode)
			}
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
			}
		}
	}
	return 0, fmt.Errorf("采集桥投递失败；游戏转发继续")
}

func (g *Gateway) deliver(ctx context.Context, d *delivery) {
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	batch := make([]Event, 0, 256)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		n, e := d.send(ctx, batch)
		if e != nil {
			g.Stats.Dropped.Add(uint64(len(batch)))
			g.Stats.LastError.Store(e.Error())
		} else {
			g.Stats.Delivered.Add(uint64(n))
			g.Stats.LastDelivery.Store(time.Now().UnixMilli())
		}
		batch = batch[:0]
	}
	for {
		select {
		case <-ctx.Done():
			g.Stats.Dropped.Add(uint64(len(batch) + len(g.queue)))
			return
		case e := <-g.queue:
			batch = append(batch, e)
			if len(batch) == 256 {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}
