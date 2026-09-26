package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/StatIndet/daybook/internal/markdown"
	"gopkg.in/yaml.v3"
)

type Options struct {
	NotesDir   string
	PublicDir  string
	ContentDir string
	ConfigPath string
	Build      func() error
}

type Server struct {
	options Options
	mu      sync.Mutex

	sessionMu sync.Mutex
	sessions  map[string]time.Time
}

const (
	sessionCookie = "daybook_admin_session"
	sessionTTL    = 7 * 24 * time.Hour
)

func New(options Options) http.Handler {
	s := &Server{options: options, sessions: map[string]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/admin", s.handlePage)
	mux.HandleFunc("/admin/", s.handlePage)
	mux.HandleFunc("/admin/api/login", s.handleLogin)
	mux.HandleFunc("/admin/api/logout", s.handleLogout)
	mux.HandleFunc("/admin/api/session", s.handleSession)
	mux.HandleFunc("/admin/api/list", s.requireAuth(s.handleList))
	mux.HandleFunc("/admin/api/note", s.requireAuth(s.handleNote))
	mux.HandleFunc("/admin/api/save", s.requireAuth(s.handleSave))
	mux.HandleFunc("/admin/api/preview", s.requireAuth(s.handlePreview))
	mux.HandleFunc("/admin/api/avatar", s.requireAuth(s.handleAvatar))
	mux.HandleFunc("/admin/api/account", s.requireAuth(s.handleAccount))
	mux.HandleFunc("/admin/api/site", s.requireAuth(s.handleSite))
	return mux
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next(w, r)
	}
}

type credentials struct {
	Username string `json:"username"`
	Salt     string `json:"salt"`
	Hash     string `json:"hash"`
}

func hashPassword(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + password))
	return hex.EncodeToString(sum[:])
}

func defaultCredentials() credentials {
	salt := "daybook-default-salt"
	return credentials{Username: "admin", Salt: salt, Hash: hashPassword(salt, "admin")}
}

func (s *Server) credentialsPath() string {
	dir := "."
	if s.options.ConfigPath != "" {
		dir = filepath.Dir(s.options.ConfigPath)
	}
	return filepath.Join(dir, ".daybook-admin.json")
}

func (s *Server) loadCredentials() credentials {
	data, err := os.ReadFile(s.credentialsPath())
	if err != nil {
		return defaultCredentials()
	}
	var c credentials
	if json.Unmarshal(data, &c) != nil || strings.TrimSpace(c.Username) == "" || c.Hash == "" || c.Salt == "" {
		return defaultCredentials()
	}
	return c
}

func (s *Server) saveCredentials(c credentials) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.credentialsPath(), data, 0600)
}

func (s *Server) newSession() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	token := hex.EncodeToString(buf)
	s.sessionMu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.sessionMu.Unlock()
	return token
}

func (s *Server) authed(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	expires, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		delete(s.sessions, cookie.Value)
		return false
	}
	return true
}

func (s *Server) dropSession(r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return
	}
	s.sessionMu.Lock()
	delete(s.sessions, cookie.Value)
	s.sessionMu.Unlock()
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	creds := s.loadCredentials()
	usernameOK := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Username)), []byte(creds.Username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(hashPassword(creds.Salt, req.Password)), []byte(creds.Hash)) == 1
	if !usernameOK || !passwordOK {
		writeError(w, http.StatusUnauthorized, "账号或密码不正确")
		return
	}
	token := s.newSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "username": creds.Username})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	s.dropSession(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	if !s.authed(r) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "authed": false})
		return
	}
	creds := s.loadCredentials()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "authed": true, "username": creds.Username})
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	creds := s.loadCredentials()
	if subtle.ConstantTimeCompare([]byte(hashPassword(creds.Salt, req.CurrentPassword)), []byte(creds.Hash)) != 1 {
		writeError(w, http.StatusUnauthorized, "当前密码不正确")
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "账号不能为空")
		return
	}
	creds.Username = username
	if strings.TrimSpace(req.NewPassword) != "" {
		saltBuf := make([]byte, 16)
		if _, err := rand.Read(saltBuf); err != nil {
			writeError(w, http.StatusInternalServerError, "生成随机盐失败")
			return
		}
		creds.Salt = hex.EncodeToString(saltBuf)
		creds.Hash = hashPassword(creds.Salt, req.NewPassword)
	}
	if err := s.saveCredentials(creds); err != nil {
		writeError(w, http.StatusInternalServerError, "保存账户失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "username": creds.Username})
}

// findConfigKey 在配置行中按嵌套路径（如 profile / author / avatar）定位键，
// 返回所在行号与行首缩进；找不到时返回 (-1, "")。
func findConfigKey(lines []string, path []string) (int, string) {
	parentIndent := -1
	start := 0
	for depth, key := range path {
		found := -1
		childIndent := -1
		for i := start; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if lineIndent <= parentIndent {
				break
			}
			if childIndent < 0 {
				childIndent = lineIndent
			}
			if lineIndent == childIndent && strings.HasPrefix(trimmed, key+":") {
				found = i
				break
			}
		}
		if found < 0 {
			return -1, ""
		}
		if depth == len(path)-1 {
			indent := lines[found][:len(lines[found])-len(strings.TrimLeft(lines[found], " \t"))]
			return found, indent
		}
		parentIndent = childIndent
		start = found + 1
	}
	return -1, ""
}

// readConfigValue 读取嵌套配置项的值（如 profile.author.avatar）。
func readConfigValue(configPath string, path ...string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	idx, _ := findConfigKey(lines, path)
	if idx < 0 {
		return ""
	}
	leaf := path[len(path)-1]
	trimmed := strings.TrimSpace(lines[idx])
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, leaf+":"))
	return strings.Trim(value, "\"'")
}

// updateConfigValue 更新嵌套配置项的值；若叶子键不存在，则在其父级块末尾插入。
func updateConfigValue(configPath, value string, path ...string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	leaf := path[len(path)-1]

	idx, indent := findConfigKey(lines, path)
	if idx >= 0 {
		lines[idx] = indent + leaf + ": " + strconv.Quote(value)
		return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0644)
	}

	if len(path) < 2 {
		return fmt.Errorf("未找到配置项 %s", strings.Join(path, "."))
	}
	parentIdx, parentIndent := findConfigKey(lines, path[:len(path)-1])
	if parentIdx < 0 {
		return fmt.Errorf("未找到配置项 %s", strings.Join(path, "."))
	}
	insertAt := parentIdx + 1
	blockIndent := parentIndent + "  "
	for i := parentIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent <= len(parentIndent) {
			break
		}
		blockIndent = line[:lineIndent]
		insertAt = i + 1
	}
	newLine := blockIndent + leaf + ": " + strconv.Quote(value)
	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:insertAt]...)
	updated = append(updated, newLine)
	updated = append(updated, lines[insertAt:]...)
	return os.WriteFile(configPath, []byte(strings.Join(updated, "\n")), 0644)
}

func (s *Server) handleSite(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"name":     readConfigValue(s.options.ConfigPath, "profile", "author", "name"),
			"nameEn":   readConfigValue(s.options.ConfigPath, "profile", "author", "nameEn"),
			"sloganZh": readConfigValue(s.options.ConfigPath, "profile", "slogan", "zh"),
			"sloganEn": readConfigValue(s.options.ConfigPath, "profile", "slogan", "en_US"),
		})
	case http.MethodPost:
		s.saveSite(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 或 POST")
	}
}

func (s *Server) saveSite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		NameEn   string `json:"nameEn"`
		SloganZh string `json:"sloganZh"`
		SloganEn string `json:"sloganEn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	if s.options.ConfigPath == "" {
		writeError(w, http.StatusInternalServerError, "服务未配置 daybook.yaml")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	updates := []struct {
		value string
		path  []string
	}{
		{strings.TrimSpace(req.Name), []string{"profile", "author", "name"}},
		{strings.TrimSpace(req.NameEn), []string{"profile", "author", "nameEn"}},
		{strings.TrimSpace(req.SloganZh), []string{"profile", "slogan", "zh"}},
		{strings.TrimSpace(req.SloganEn), []string{"profile", "slogan", "en_US"}},
	}
	for _, u := range updates {
		if err := updateConfigValue(s.options.ConfigPath, u.value, u.path...); err != nil {
			writeError(w, http.StatusInternalServerError, "更新 daybook.yaml 失败: "+err.Error())
			return
		}
	}
	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "已保存，但重建站点失败: "+buildErr.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "avatar": readConfigValue(s.options.ConfigPath, "profile", "author", "avatar")})
	case http.MethodPost:
		s.uploadAvatar(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 或 POST")
	}
}

func (s *Server) removeOtherAvatars(keep string) {
	for _, dir := range []string{s.options.ContentDir, s.options.PublicDir} {
		if dir == "" {
			continue
		}
		var matches []string
		for _, pattern := range []string{"avatar.*", "avatar-*"} {
			if m, err := filepath.Glob(filepath.Join(dir, pattern)); err == nil {
				matches = append(matches, m...)
			}
		}
		for _, match := range matches {
			if filepath.Base(match) == keep {
				continue
			}
			_ = os.Remove(match)
		}
	}
}

func (s *Server) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "上传内容过大或格式错误")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "没有收到图片文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg":
	default:
		writeError(w, http.StatusBadRequest, "仅支持 png / jpg / jpeg / gif / webp / avif / svg 图片")
		return
	}
	if s.options.ContentDir == "" || s.options.ConfigPath == "" {
		writeError(w, http.StatusInternalServerError, "服务未配置内容目录")
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取图片失败: "+err.Error())
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "图片内容为空")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	name := "avatar-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	target := filepath.Join(s.options.ContentDir, name)
	if err := os.MkdirAll(s.options.ContentDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "创建目录失败: "+err.Error())
		return
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "保存图片失败: "+err.Error())
		return
	}
	s.removeOtherAvatars(name)

	webPath := "/" + name
	if err := updateConfigValue(s.options.ConfigPath, webPath, "profile", "author", "avatar"); err != nil {
		writeError(w, http.StatusInternalServerError, "更新 daybook.yaml 失败: "+err.Error())
		return
	}
	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "头像已保存，但重建站点失败: "+buildErr.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "avatar": webPath})
}

type frontmatter struct {
	Title   string   `yaml:"title"`
	Date    string   `yaml:"date"`
	Updated string   `yaml:"updated,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Summary string   `yaml:"summary,omitempty"`
	Lang    string   `yaml:"lang,omitempty"`
	I18nKey string   `yaml:"i18n_key,omitempty"`
	Draft   bool     `yaml:"draft,omitempty"`
	Math    bool     `yaml:"math,omitempty"`
}

func splitFrontmatter(text string) (string, string, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			body := strings.Join(lines[i+1:], "\n")
			body = strings.TrimLeft(body, "\r\n")
			body = strings.TrimRight(body, " \t\r\n")
			return strings.Join(lines[1:i], "\n"), body, true
		}
	}
	return "", "", false
}

var slugPattern = regexp.MustCompile(`[\\/:*?"<>|]+`)

func sanitizeSlug(slug string) string {
	slug = strings.TrimSpace(slug)
	slug = slugPattern.ReplaceAllString(slug, "-")
	slug = strings.Join(strings.Fields(slug), "-")
	slug = strings.Trim(slug, "-. ")
	return slug
}

func (s *Server) notePath(slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("链接标识不能为空")
	}
	if strings.ContainsAny(slug, `/\`) || strings.Contains(slug, "..") {
		return "", fmt.Errorf("链接标识不能包含路径分隔符")
	}
	target := filepath.Join(s.options.NotesDir, slug+".md")
	rel, err := filepath.Rel(s.options.NotesDir, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("链接标识非法")
	}
	return target, nil
}

func (s *Server) manifest() map[string]string {
	data, err := os.ReadFile(filepath.Join(s.options.PublicDir, "assets-manifest.json"))
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": message})
}

func (s *Server) assetLinks() (func(string) string, string) {
	manifest := s.manifest()
	render := func(webPath string) string {
		if manifest != nil {
			if resolved, ok := manifest[webPath]; ok && resolved != "" {
				return resolved
			}
		}
		return webPath
	}
	var fonts strings.Builder
	for _, p := range []string{
		"/vendor/fonts/lxgw-wenkai-screen/regular/result.css",
		"/vendor/fonts/maple-mono-cn/regular/result.css",
		"/vendor/fonts/maple-mono-cn/italic/result.css",
		"/vendor/fonts/noto-serif-sc/wght.css",
		"/css/decorative-fonts.css",
	} {
		fonts.WriteString("<link rel=\"stylesheet\" href=\"" + render(p) + "\">\n")
	}
	return render, fonts.String()
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	render, fonts := s.assetLinks()
	template := pageTemplate
	if !s.authed(r) {
		template = loginTemplate
	}
	page := strings.Replace(template, "{{GLOBAL_CSS}}", render("/css/global.css"), 1)
	page = strings.Replace(page, "{{FONT_LINKS}}", fonts, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

type listItem struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Lang    string   `json:"lang"`
	I18nKey string   `json:"i18nKey"`
	Draft   bool     `json:"draft"`
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	items := []listItem{}
	err := filepath.WalkDir(s.options.NotesDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(s.options.NotesDir, path)
		if relErr != nil {
			return nil
		}
		slug := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		item := listItem{Slug: slug, Title: slug, Lang: "zh_CN"}
		if data, readErr := os.ReadFile(path); readErr == nil {
			if yamlText, _, ok := splitFrontmatter(string(data)); ok {
				var fm frontmatter
				if yaml.Unmarshal([]byte(yamlText), &fm) == nil {
					if strings.TrimSpace(fm.Title) != "" {
						item.Title = strings.TrimSpace(fm.Title)
					}
					item.Date = strings.TrimSpace(fm.Date)
					item.Tags = fm.Tags
					if lang := strings.TrimSpace(fm.Lang); lang != "" {
						item.Lang = lang
					}
					item.I18nKey = strings.TrimSpace(fm.I18nKey)
					item.Draft = fm.Draft
				}
			}
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取文章列表失败: "+err.Error())
		return
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date > items[j].Date
		}
		return items[i].Slug < items[j].Slug
	})
	writeJSON(w, http.StatusOK, map[string]any{"notes": items})
}

type notePayload struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
	Lang    string   `json:"lang"`
	I18nKey string   `json:"i18nKey"`
	Draft   bool     `json:"draft"`
	Body    string   `json:"body"`
	IsNew   bool     `json:"isNew"`
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		writeJSON(w, http.StatusOK, notePayload{IsNew: true, Lang: "zh_CN", Date: time.Now().Format("2006-01-02")})
		return
	}
	path, pathErr := s.notePath(slug)
	if pathErr != nil {
		writeError(w, http.StatusBadRequest, pathErr.Error())
		return
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		writeError(w, http.StatusNotFound, "找不到该文章")
		return
	}
	payload := notePayload{Slug: slug}
	if yamlText, body, ok := splitFrontmatter(string(data)); ok {
		var fm frontmatter
		if yaml.Unmarshal([]byte(yamlText), &fm) == nil {
			payload.Title = strings.TrimSpace(fm.Title)
			payload.Date = strings.TrimSpace(fm.Date)
			payload.Tags = fm.Tags
			payload.Summary = strings.TrimSpace(fm.Summary)
			payload.Lang = strings.TrimSpace(fm.Lang)
			payload.I18nKey = strings.TrimSpace(fm.I18nKey)
			payload.Draft = fm.Draft
			payload.Body = body
		}
	} else {
		payload.Body = string(data)
	}
	if payload.Lang == "" {
		payload.Lang = "zh_CN"
	}
	writeJSON(w, http.StatusOK, payload)
}

type saveRequest struct {
	OriginalSlug string   `json:"originalSlug"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Date         string   `json:"date"`
	Tags         []string `json:"tags"`
	Summary      string   `json:"summary"`
	Lang         string   `json:"lang"`
	I18nKey      string   `json:"i18nKey"`
	Draft        bool     `json:"draft"`
	Body         string   `json:"body"`
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "标题不能为空")
		return
	}
	date := strings.TrimSpace(req.Date)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	lang := strings.TrimSpace(req.Lang)
	if lang == "" {
		lang = "zh_CN"
	}
	if lang != "zh_CN" && lang != "en_US" {
		writeError(w, http.StatusBadRequest, "语言只能是 zh_CN 或 en_US")
		return
	}

	slug := sanitizeSlug(req.Slug)
	if slug == "" {
		slug = time.Now().Format("2006-01-02-150405")
	}
	original := sanitizeSlug(req.OriginalSlug)

	target, pathErr := s.notePath(slug)
	if pathErr != nil {
		writeError(w, http.StatusBadRequest, pathErr.Error())
		return
	}
	if slug != original {
		if _, statErr := os.Stat(target); statErr == nil {
			writeError(w, http.StatusConflict, "已存在同名文章，请更换链接标识")
			return
		}
	}

	var tags []string
	for _, tag := range req.Tags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	fm := frontmatter{
		Title:   title,
		Date:    date,
		Tags:    tags,
		Summary: strings.TrimSpace(req.Summary),
		Lang:    lang,
		I18nKey: strings.TrimSpace(req.I18nKey),
		Draft:   req.Draft,
	}
	yamlBytes, marshalErr := yaml.Marshal(fm)
	if marshalErr != nil {
		writeError(w, http.StatusInternalServerError, "生成 frontmatter 失败: "+marshalErr.Error())
		return
	}
	body := strings.TrimRight(req.Body, " \t\r\n")
	content := "---\n" + string(yamlBytes) + "---\n\n"
	if body != "" {
		content += body + "\n"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "创建目录失败: "+err.Error())
		return
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "保存文章失败: "+err.Error())
		return
	}
	if original != "" && original != slug {
		if oldPath, oldErr := s.notePath(original); oldErr == nil {
			_ = os.Remove(oldPath)
		}
	}
	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "文章已保存，但重建站点失败: "+buildErr.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"slug": slug,
		"url":  "/notes/" + slug + "/",
	})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	html, renderErr := markdown.ToHTML(req.Body)
	if renderErr != nil {
		writeError(w, http.StatusInternalServerError, "渲染失败: "+renderErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"html": html})
}
