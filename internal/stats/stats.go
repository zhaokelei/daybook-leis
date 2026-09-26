package stats

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const visitorCookie = "daybook_visitor"

type persisted struct {
	PageViews map[string]int `json:"pageViews"`
	Visitors  []string       `json:"visitors"`
	Total     int            `json:"totalViews"`
}

// Store 保存浏览量统计，并持久化到工作目录下的 .daybook-stats.json。
type Store struct {
	mu        sync.Mutex
	pageViews map[string]int
	visitors  map[string]struct{}
	total     int
	filePath  string
}

func NewStore(dir string) *Store {
	s := &Store{
		pageViews: map[string]int{},
		visitors:  map[string]struct{}{},
		filePath:  filepath.Join(dir, ".daybook-stats.json"),
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var p persisted
	if json.Unmarshal(data, &p) != nil {
		return
	}
	if p.PageViews != nil {
		s.pageViews = p.PageViews
	}
	for _, v := range p.Visitors {
		s.visitors[v] = struct{}{}
	}
	s.total = p.Total
}

func (s *Store) saveLocked() {
	p := persisted{
		PageViews: s.pageViews,
		Visitors:  make([]string, 0, len(s.visitors)),
		Total:     s.total,
	}
	for v := range s.visitors {
		p.Visitors = append(p.Visitors, v)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filePath, data, 0644)
}

type hitResponse struct {
	Path       string `json:"path"`
	PageViews  int    `json:"pageViews"`
	TotalViews int    `json:"totalViews"`
	Visitors   int    `json:"visitors"`
}

// Hit 记录一次页面访问，返回该路径与全站的统计数据。
func (s *Store) Hit(path, visitorID string, isNew bool) hitResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pageViews[path]++
	s.total++
	if isNew && visitorID != "" {
		s.visitors[visitorID] = struct{}{}
	}
	s.saveLocked()

	return hitResponse{
		Path:       path,
		PageViews:  s.pageViews[path],
		TotalViews: s.total,
		Visitors:   len(s.visitors),
	}
}

type client struct {
	ws   *websocket.Conn
	path string
	send chan []byte
}

type presenceHub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

func newPresenceHub() *presenceHub {
	return &presenceHub{clients: map[*client]struct{}{}}
}

func (h *presenceHub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.broadcast()
}

func (h *presenceHub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	h.broadcast()
}

func (h *presenceHub) setPath(c *client, path string) {
	h.mu.Lock()
	if _, ok := h.clients[c]; !ok {
		h.mu.Unlock()
		return
	}
	c.path = path
	h.mu.Unlock()
	h.broadcast()
}

type presenceMessage struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	PageViewers int    `json:"pageViewers"`
	SiteViewers int    `json:"siteViewers"`
}

func (h *presenceHub) broadcast() {
	h.mu.Lock()
	counts := map[string]int{}
	for c := range h.clients {
		counts[c.path]++
	}
	site := len(h.clients)
	payloads := map[*client][]byte{}
	for c := range h.clients {
		msg := presenceMessage{Type: "presence", Path: c.path, PageViewers: counts[c.path], SiteViewers: site}
		if data, err := json.Marshal(msg); err == nil {
			payloads[c] = data
		}
	}
	h.mu.Unlock()

	for c, data := range payloads {
		select {
		case c.send <- data:
		default:
		}
	}
}

// Handler 提供 /api/hit 与 /api/presence 两个接口。
type Handler struct {
	store    *Store
	presence *presenceHub
}

func New(dir string) *Handler {
	return &Handler{store: NewStore(dir), presence: newPresenceHub()}
}

// NormalizePath 与前端 normalizePath 行为保持一致。
func NormalizePath(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return "/"
	}
	if idx := strings.IndexAny(input, "?#"); idx >= 0 {
		input = input[:idx]
	}
	if decoded, err := url.PathUnescape(input); err == nil {
		input = decoded
	}
	for strings.Contains(input, "//") {
		input = strings.ReplaceAll(input, "//", "/")
	}
	if !strings.HasPrefix(input, "/") {
		input = "/" + input
	}
	if input != "/" && !strings.HasSuffix(input, "/") {
		input += "/"
	}
	return input
}

// HandleHit 处理 POST /api/hit。
func (h *Handler) HandleHit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "只支持 POST"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请求格式错误"})
		return
	}
	path := NormalizePath(req.Path)

	visitorID, isNew := ensureVisitor(w, r)
	resp := h.store.Hit(path, visitorID, isNew)
	writeJSON(w, http.StatusOK, resp)
}

func ensureVisitor(w http.ResponseWriter, r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(visitorCookie); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return cookie.Value, false
	}
	id := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     visitorCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
	})
	return id, true
}

func randomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// HandlePresence 处理 WS /api/presence。
func (h *Handler) HandlePresence(w http.ResponseWriter, r *http.Request) {
	path := NormalizePath(r.URL.Query().Get("path"))
	websocket.Handler(func(ws *websocket.Conn) {
		c := &client{ws: ws, path: path, send: make(chan []byte, 8)}
		h.presence.register(c)

		done := make(chan struct{})
		go func() {
			for data := range c.send {
				if _, err := ws.Write(data); err != nil {
					break
				}
			}
			close(done)
		}()

		buf := make([]byte, 4096)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				break
			}
			var msg struct {
				Type string `json:"type"`
				Path string `json:"path"`
			}
			if json.Unmarshal(buf[:n], &msg) == nil && msg.Type == "navigate" && msg.Path != "" {
				h.presence.setPath(c, NormalizePath(msg.Path))
			}
		}

		h.presence.unregister(c)
		close(c.send)
		<-done
	}).ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
