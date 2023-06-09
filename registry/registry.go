package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// FastRegistry is a simple register center, provide following functions.
// add a server and receive heartbeat to keep it alive.
// returns all alive servers and delete dead servers sync simultaneously.
type FastRegistry struct {
	timeout time.Duration
	mu      sync.Mutex // protect following
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr  string
	start time.Time // 服务实例的注册时间
}

const (
	defaultPath    = "/fastrpc/registry"
	defaultTimeout = time.Minute * 5
)

// New create a registry instance with timeout setting
func New(timeout time.Duration) *FastRegistry {
	return &FastRegistry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

var DefaultFastRegister = New(defaultTimeout)

// 为 FastRegistry 实现添加服务实例和返回服务列表的方法：
// 1. putServer：添加服务实例，如果服务已经存在，则更新 start。
// 2. aliveServers：返回可用的服务列表，如果存在超时的服务，则删除。
func (r *FastRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		s.start = time.Now() // if exists, update start time to keep alive
	}
}

func (r *FastRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

// 为了实现上的简单，FastRegistry 采用 HTTP 协议提供服务，且所有的有用信息都承载在 HTTP Header 中：
// 1. Get：返回所有可用的服务列表，通过自定义字段 X-FastRPC-Servers 承载。
// 2. Post：添加服务实例或发送心跳，通过自定义字段 X-FastRPC-Server 承载。
// Runs at /fastrpc/registry
func (r *FastRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// keep it simple, server is in req.Header
		w.Header().Set("X-FastRPC-Servers", strings.Join(r.aliveServers(), ","))
	case http.MethodPost:
		// keep it simple, server is in req.Header
		addr := req.Header.Get("X-FastRPC-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleHTTP registers an HTTP handler for GeeRegistry messages on registryPath
func (r *FastRegistry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("FastRPC registry path:", registryPath)
}

func HandleHTTP() {
	DefaultFastRegister.HandleHTTP(defaultPath)
}

// Heartbeat send a heartbeat message every once in a while
// it's a helper function for a server to register or send heartbeat
// 便于服务启动时定时向注册中心发送心跳，默认周期比注册中心设置的过期时间少 1 min
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		// make sure there is enough time to send heart beat before it's removed from registry
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-FastRPC-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("FastRPC server: heart beat err:", err)
		return err
	}
	return nil
}
