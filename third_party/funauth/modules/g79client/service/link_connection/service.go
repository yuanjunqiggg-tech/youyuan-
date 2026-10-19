package link_connection

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	g79client "github.com/Yeah114/g79client"
)

// LinkEndpoint 描述一个可用的 Link 服务器节点。
type LinkEndpoint struct {
	Entry g79client.G79LinkServerEntry
}

// LinkConnectionService 管理 Link 服务器列表与连接创建。
type LinkConnectionService struct {
	client *g79client.Client

	mu      sync.RWMutex
	servers []g79client.G79LinkServerEntry

	randMu sync.Mutex
	rnd    *rand.Rand
}

// NewLinkConnectionService 根据客户端信息创建服务。
func NewLinkConnectionService(client *g79client.Client) (*LinkConnectionService, error) {
	return NewLinkConnectionServiceContext(context.Background(), client)
}

// NewLinkConnectionServiceContext discovers the current official list within ctx.
func NewLinkConnectionServiceContext(ctx context.Context, client *g79client.Client) (*LinkConnectionService, error) {
	if client == nil {
		return nil, errors.New("link_connection.NewLinkConnectionService: client 不能为空")
	}

	svc := &LinkConnectionService{
		client: client,
		rnd:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := svc.primeServers(ctx); err != nil {
		// 允许延迟错误返回给调用方
		return svc, err
	}
	return svc, nil
}

func (s *LinkConnectionService) primeServers(ctx context.Context) error {
	list, err := s.client.FetchG79LinkServersContext(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errors.New("link_connection: official link list is empty")
	}
	s.mu.Lock()
	s.servers = cloneLinkServers(list)
	s.mu.Unlock()
	return nil
}

// Client 返回底层 G79 客户端。
func (s *LinkConnectionService) Client() *g79client.Client {
	return s.client
}

// Servers 返回缓存的服务器列表，如果缓存为空则尝试刷新。
func (s *LinkConnectionService) Servers() ([]g79client.G79LinkServerEntry, error) {
	return s.ServersContext(context.Background())
}

func (s *LinkConnectionService) ServersContext(ctx context.Context) ([]g79client.G79LinkServerEntry, error) {
	s.mu.RLock()
	if s.servers != nil {
		list := cloneLinkServers(s.servers)
		s.mu.RUnlock()
		return list, nil
	}
	s.mu.RUnlock()
	return s.RefreshServersContext(ctx)
}

// RefreshServers 强制刷新服务器列表缓存。
func (s *LinkConnectionService) RefreshServers() ([]g79client.G79LinkServerEntry, error) {
	return s.RefreshServersContext(context.Background())
}

func (s *LinkConnectionService) RefreshServersContext(ctx context.Context) ([]g79client.G79LinkServerEntry, error) {
	list, err := s.client.FetchG79LinkServersContext(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.servers = cloneLinkServers(list)
	s.mu.Unlock()
	return cloneLinkServers(list), nil
}

// SelectEndpoint 随机选择一个可用节点。
func (s *LinkConnectionService) SelectEndpoint() (*LinkEndpoint, error) {
	servers, err := s.Servers()
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, errors.New("link_connection.SelectEndpoint: 当前没有可用的 link 服务器")
	}
	entry := s.pickRandom(servers)
	return &LinkEndpoint{Entry: entry}, nil
}

// Dial tries at most three distinct official endpoints. Only a refused TCP
// connection is safe to fail over: handshake/login errors are returned as-is.
func (s *LinkConnectionService) Dial(ctx context.Context) (*LinkConnection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	servers, err := s.ServersContext(ctx)
	if err != nil {
		return nil, err
	}
	plan := s.candidatePlan(servers)
	if len(plan) == 0 {
		return nil, errors.New("link_connection.Dial: 当前没有可用的 link 服务器")
	}
	return dialLinkCandidates(ctx, plan, s.DialEndpoint)
}

func dialLinkCandidates(ctx context.Context, plan []g79client.G79LinkServerEntry, dial func(context.Context, *LinkEndpoint) (*LinkConnection, error)) (*LinkConnection, error) {
	var lastErr error
	for i, entry := range plan {
		if i == 3 {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := dial(ctx, &LinkEndpoint{Entry: entry})
		if err == nil {
			return conn, nil
		}
		lastErr = err
		var dialErr *linkTCPDialError
		if !errors.As(err, &dialErr) || !isConnectionRefused(err) {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, lastErr
}

type linkTCPDialError struct{ error }

func (e *linkTCPDialError) Unwrap() error { return e.error }

// Interleave server types to spread transport failures without interpreting
// status or excluding any type. Shuffle first, then visit each type in turn.
func (s *LinkConnectionService) candidatePlan(servers []g79client.G79LinkServerEntry) []g79client.G79LinkServerEntry {
	list := make([]g79client.G79LinkServerEntry, 0, len(servers))
	seen := map[string]bool{}
	for _, entry := range servers {
		port := entry.Port.Int64()
		key := net.JoinHostPort(entry.IP, strconv.FormatInt(port, 10))
		if entry.IP == "" || port < 1 || port > 65535 || seen[key] {
			continue
		}
		seen[key] = true
		list = append(list, entry)
	}
	s.randMu.Lock()
	s.rnd.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	s.randMu.Unlock()
	groups := map[string][]g79client.G79LinkServerEntry{}
	var types []string
	for _, entry := range list {
		if _, ok := groups[entry.ServerType]; !ok {
			types = append(types, entry.ServerType)
		}
		groups[entry.ServerType] = append(groups[entry.ServerType], entry)
	}
	plan := make([]g79client.G79LinkServerEntry, 0, 3)
	for len(plan) < 3 {
		added := false
		for _, typ := range types {
			group := groups[typ]
			if len(group) == 0 {
				continue
			}
			plan = append(plan, group[0])
			groups[typ] = group[1:]
			added = true
			if len(plan) == 3 {
				break
			}
		}
		if !added {
			break
		}
	}
	return plan
}

// DialEndpoint 使用指定节点建立连接。
func (s *LinkConnectionService) DialEndpoint(ctx context.Context, endpoint *LinkEndpoint) (*LinkConnection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if endpoint == nil {
		return nil, errors.New("link_connection.DialEndpoint: endpoint 不能为空")
	}

	d := &net.Dialer{}
	if deadline, ok := ctx.Deadline(); ok {
		d.Timeout = time.Until(deadline)
	}

	port := int(endpoint.Entry.Port.Int64())
	if port < 1 || port > 65535 {
		return nil, errors.New("link_connection.DialEndpoint: 节点端口无效")
	}
	addr := net.JoinHostPort(endpoint.Entry.IP, strconv.Itoa(port))
	dialContext := d.DialContext
	if injected, ok := ctx.Value(linkDialerContextKey{}).(func(context.Context, string, string) (net.Conn, error)); ok && injected != nil {
		dialContext = injected
	}
	conn, err := dialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, &linkTCPDialError{err}
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancel()

	linkConn, err := newLinkConnection(s, conn, endpoint.Entry)
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	if err := linkConn.waitForLogin(ctx); err != nil {
		linkConn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		linkConn.Close()
		return nil, err
	}
	return linkConn, nil
}

func (s *LinkConnectionService) pickRandom(servers []g79client.G79LinkServerEntry) g79client.G79LinkServerEntry {
	if len(servers) == 1 {
		return servers[0]
	}
	s.randMu.Lock()
	idx := s.rnd.Intn(len(servers))
	s.randMu.Unlock()
	return servers[idx]
}

func cloneLinkServers(src []g79client.G79LinkServerEntry) []g79client.G79LinkServerEntry {
	if len(src) == 0 {
		return nil
	}
	dst := make([]g79client.G79LinkServerEntry, len(src))
	copy(dst, src)
	return dst
}
