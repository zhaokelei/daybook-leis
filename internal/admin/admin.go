package admin

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
	mux.HandleFunc("/admin/api/delete", s.requireAuth(s.handleDelete))
	mux.HandleFunc("/admin/api/preview", s.requireAuth(s.handlePreview))
	mux.HandleFunc("/admin/api/avatar", s.requireAuth(s.handleAvatar))
	mux.HandleFunc("/admin/api/account", s.requireAuth(s.handleAccount))
	mux.HandleFunc("/admin/api/site", s.requireAuth(s.handleSite))
	mux.HandleFunc("/admin/api/social", s.requireAuth(s.handleSocial))
	mux.HandleFunc("/admin/api/export", s.requireAuth(s.handleExport))
	mux.HandleFunc("/admin/api/import", s.requireAuth(s.handleImport))
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

// readConfigBool 读取布尔型配置项。
func readConfigBool(configPath string, path ...string) bool {
	return strings.EqualFold(readConfigValue(configPath, path...), "true")
}

// readConfigInt 读取整型配置项，解析失败时返回 0。
func readConfigInt(configPath string, path ...string) int {
	n, err := strconv.Atoi(readConfigValue(configPath, path...))
	if err != nil {
		return 0
	}
	return n
}

// updateConfigValue 更新嵌套配置项的值（自动加引号）；若叶子键不存在，则在其父级块末尾插入。
func updateConfigValue(configPath, value string, path ...string) error {
	return writeConfigLeaf(configPath, strconv.Quote(value), path...)
}

// updateConfigRawValue 以原始文本写入配置项（不加引号），用于布尔值与数字。
func updateConfigRawValue(configPath, raw string, path ...string) error {
	return writeConfigLeaf(configPath, raw, path...)
}

// writeConfigLeaf 将 formatted 作为配置项的值写入；若叶子键不存在，则在其父级块末尾插入。
func writeConfigLeaf(configPath, formatted string, path ...string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	leaf := path[len(path)-1]

	idx, indent := findConfigKey(lines, path)
	if idx >= 0 {
		lines[idx] = indent + leaf + ": " + formatted
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
	newLine := blockIndent + leaf + ": " + formatted
	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:insertAt]...)
	updated = append(updated, newLine)
	updated = append(updated, lines[insertAt:]...)
	return os.WriteFile(configPath, []byte(strings.Join(updated, "\n")), 0644)
}

func (s *Server) handleSite(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.options.ConfigPath
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":                  true,
			"name":                readConfigValue(cfg, "profile", "author", "name"),
			"nameEn":              readConfigValue(cfg, "profile", "author", "nameEn"),
			"sloganZh":            readConfigValue(cfg, "profile", "slogan", "zh"),
			"sloganEn":            readConfigValue(cfg, "profile", "slogan", "en_US"),
			"siteUrl":             readConfigValue(cfg, "site", "url"),
			"siteTitleZh":         readConfigValue(cfg, "site", "name", "zh"),
			"siteTitleEn":         readConfigValue(cfg, "site", "name", "en"),
			"startedAt":           readConfigValue(cfg, "site", "startedAt"),
			"copyright":           readConfigValue(cfg, "site", "copyright"),
			"favicon":             readConfigValue(cfg, "site", "favicon"),
			"logoText":            readConfigValue(cfg, "profile", "author", "logoText"),
			"aboutUrl":            readConfigValue(cfg, "profile", "author", "aboutUrl"),
			"homeTitleZh":         readConfigValue(cfg, "seo", "homeTitle", "zh"),
			"homeTitleEn":         readConfigValue(cfg, "seo", "homeTitle", "en"),
			"homeDescZh":          readConfigValue(cfg, "seo", "homeDescription", "zh"),
			"homeDescEn":          readConfigValue(cfg, "seo", "homeDescription", "en"),
			"shareText":           readConfigValue(cfg, "share", "text"),
			"statsEnabled":        readConfigBool(cfg, "stats", "enabled"),
			"commentEnabled":      readConfigBool(cfg, "comment", "enabled"),
			"commentProvider":     readConfigValue(cfg, "comment", "provider"),
			"walineServerUrl":     readConfigValue(cfg, "comment", "waline", "serverURL"),
			"walineLang":          readConfigValue(cfg, "comment", "waline", "lang"),
			"walinePageSize":      readConfigInt(cfg, "comment", "waline", "pageSize"),
			"walineSorting":       readConfigValue(cfg, "comment", "waline", "commentSorting"),
			"walineSearch":        readConfigBool(cfg, "comment", "waline", "search"),
			"walineImageUploader": readConfigBool(cfg, "comment", "waline", "imageUploader"),
		})
	case http.MethodPost:
		s.saveSite(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 或 POST")
	}
}

func (s *Server) saveSite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                string `json:"name"`
		NameEn              string `json:"nameEn"`
		SloganZh            string `json:"sloganZh"`
		SloganEn            string `json:"sloganEn"`
		SiteURL             string `json:"siteUrl"`
		SiteTitleZh         string `json:"siteTitleZh"`
		SiteTitleEn         string `json:"siteTitleEn"`
		StartedAt           string `json:"startedAt"`
		Copyright           string `json:"copyright"`
		Favicon             string `json:"favicon"`
		LogoText            string `json:"logoText"`
		AboutURL            string `json:"aboutUrl"`
		HomeTitleZh         string `json:"homeTitleZh"`
		HomeTitleEn         string `json:"homeTitleEn"`
		HomeDescZh          string `json:"homeDescZh"`
		HomeDescEn          string `json:"homeDescEn"`
		ShareText           string `json:"shareText"`
		StatsEnabled        bool   `json:"statsEnabled"`
		CommentEnabled      bool   `json:"commentEnabled"`
		CommentProvider     string `json:"commentProvider"`
		WalineServerURL     string `json:"walineServerUrl"`
		WalineLang          string `json:"walineLang"`
		WalinePageSize      int    `json:"walinePageSize"`
		WalineSorting       string `json:"walineSorting"`
		WalineSearch        bool   `json:"walineSearch"`
		WalineImageUploader bool   `json:"walineImageUploader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	if s.options.ConfigPath == "" {
		writeError(w, http.StatusInternalServerError, "服务未配置 daybook.yaml")
		return
	}

	siteURL := strings.TrimSpace(req.SiteURL)
	if siteURL != "" && !strings.HasPrefix(siteURL, "http://") && !strings.HasPrefix(siteURL, "https://") {
		writeError(w, http.StatusBadRequest, "站点网址必须以 http:// 或 https:// 开头")
		return
	}

	pageSize := req.WalinePageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	updates := []struct {
		value string
		raw   bool
		path  []string
	}{
		{strings.TrimSpace(req.Name), false, []string{"profile", "author", "name"}},
		{strings.TrimSpace(req.NameEn), false, []string{"profile", "author", "nameEn"}},
		{strings.TrimSpace(req.SloganZh), false, []string{"profile", "slogan", "zh"}},
		{strings.TrimSpace(req.SloganEn), false, []string{"profile", "slogan", "en_US"}},
		{siteURL, false, []string{"site", "url"}},
		{strings.TrimSpace(req.SiteTitleZh), false, []string{"site", "name", "zh"}},
		{strings.TrimSpace(req.SiteTitleEn), false, []string{"site", "name", "en"}},
		{strings.TrimSpace(req.StartedAt), false, []string{"site", "startedAt"}},
		{strings.TrimSpace(req.Copyright), false, []string{"site", "copyright"}},
		{strings.TrimSpace(req.Favicon), false, []string{"site", "favicon"}},
		{strings.TrimSpace(req.LogoText), false, []string{"profile", "author", "logoText"}},
		{strings.TrimSpace(req.AboutURL), false, []string{"profile", "author", "aboutUrl"}},
		{strings.TrimSpace(req.HomeTitleZh), false, []string{"seo", "homeTitle", "zh"}},
		{strings.TrimSpace(req.HomeTitleEn), false, []string{"seo", "homeTitle", "en"}},
		{strings.TrimSpace(req.HomeDescZh), false, []string{"seo", "homeDescription", "zh"}},
		{strings.TrimSpace(req.HomeDescEn), false, []string{"seo", "homeDescription", "en"}},
		{strings.TrimSpace(req.ShareText), false, []string{"share", "text"}},
		{strconv.FormatBool(req.StatsEnabled), true, []string{"stats", "enabled"}},
		{strconv.FormatBool(req.CommentEnabled), true, []string{"comment", "enabled"}},
		{strings.TrimSpace(req.CommentProvider), false, []string{"comment", "provider"}},
		{strings.TrimSpace(req.WalineServerURL), false, []string{"comment", "waline", "serverURL"}},
		{strings.TrimSpace(req.WalineLang), false, []string{"comment", "waline", "lang"}},
		{strconv.Itoa(pageSize), true, []string{"comment", "waline", "pageSize"}},
		{strings.TrimSpace(req.WalineSorting), false, []string{"comment", "waline", "commentSorting"}},
		{strconv.FormatBool(req.WalineSearch), true, []string{"comment", "waline", "search"}},
		{strconv.FormatBool(req.WalineImageUploader), true, []string{"comment", "waline", "imageUploader"}},
	}
	for _, u := range updates {
		var err error
		if u.raw {
			err = updateConfigRawValue(s.options.ConfigPath, u.value, u.path...)
		} else {
			err = updateConfigValue(s.options.ConfigPath, u.value, u.path...)
		}
		if err != nil {
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

// socialEntry 对应 daybook.yaml 中 profile.social 数组的单项。
type socialEntry struct {
	Type string `yaml:"type" json:"type"`
	URL  string `yaml:"url" json:"url"`
}

// socialConfigFile 仅用于读取 daybook.yaml 中的 profile.social。
type socialConfigFile struct {
	Profile struct {
		Social []socialEntry `yaml:"social"`
	} `yaml:"profile"`
}

func (s *Server) handleSocial(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		links := []socialEntry{}
		if data, err := os.ReadFile(s.options.ConfigPath); err == nil {
			var cfg socialConfigFile
			if yaml.Unmarshal(data, &cfg) == nil {
				links = cfg.Profile.Social
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "links": links})
	case http.MethodPost:
		s.saveSocial(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 或 POST")
	}
}

func (s *Server) saveSocial(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Links []socialEntry `json:"links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	var links []socialEntry
	for _, l := range req.Links {
		l.Type = strings.ToLower(strings.TrimSpace(l.Type))
		l.URL = strings.TrimSpace(l.URL)
		if l.Type == "" || l.URL == "" {
			continue
		}
		links = append(links, l)
	}
	if err := updateSocialLinks(s.options.ConfigPath, links); err != nil {
		writeError(w, http.StatusInternalServerError, "保存社交链接失败: "+err.Error())
		return
	}
	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "已保存，但重建站点失败: "+buildErr.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// updateSocialLinks 替换 daybook.yaml 中 profile.social 的整个数组块，
// 保留其余配置不变。links 为空时写为 social: []。
func updateSocialLinks(configPath string, links []socialEntry) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	socialIdx, socialIndent := findConfigKey(lines, []string{"profile", "social"})
	if socialIdx < 0 {
		return fmt.Errorf("未找到 profile.social 配置项")
	}

	// 找到 social 块的结束位置：下一个缩进 <= social 缩进的非注释非空行。
	endIdx := len(lines)
	for i := socialIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent <= len(socialIndent) {
			endIdx = i
			break
		}
	}

	var newLines []string
	if len(links) == 0 {
		newLines = append(newLines, socialIndent+"social: []")
	} else {
		newLines = append(newLines, socialIndent+"social:")
		itemIndent := socialIndent + "  "
		for _, l := range links {
			newLines = append(newLines, itemIndent+"- type: "+strconv.Quote(l.Type))
			newLines = append(newLines, itemIndent+"  url: "+strconv.Quote(l.URL))
		}
	}

	updated := make([]string, 0, len(lines))
	updated = append(updated, lines[:socialIdx]...)
	updated = append(updated, newLines...)
	updated = append(updated, lines[endIdx:]...)
	return os.WriteFile(configPath, []byte(strings.Join(updated, "\n")), 0644)
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
	Pin     bool     `yaml:"pin,omitempty"`
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
	Pin     bool     `json:"pin"`
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
					item.Pin = fm.Pin
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
	Pin     bool     `json:"pin"`
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
			payload.Pin = fm.Pin
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
	Pin          bool     `json:"pin"`
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
		Pin:     req.Pin,
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

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	slug := sanitizeSlug(req.Slug)
	if slug == "" {
		writeError(w, http.StatusBadRequest, "缺少链接标识")
		return
	}
	target, pathErr := s.notePath(slug)
	if pathErr != nil {
		writeError(w, http.StatusBadRequest, pathErr.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := os.Stat(target); statErr != nil {
		writeError(w, http.StatusNotFound, "文章不存在")
		return
	}
	if err := os.Remove(target); err != nil {
		writeError(w, http.StatusInternalServerError, "删除文章失败: "+err.Error())
		return
	}
	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "文章已删除，但重建站点失败: "+buildErr.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"slug": slug,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	if s.options.ConfigPath == "" {
		writeError(w, http.StatusInternalServerError, "未配置站点路径，无法导出")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filename := "daybook-backup-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Cache-Control", "no-store")

	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := addFileToZip(zw, s.options.ConfigPath, "daybook.yaml"); err != nil {
		return
	}
	if s.options.ContentDir != "" {
		_ = addDirToZip(zw, s.options.ContentDir, "vault")
	}
}

func addFileToZip(zw *zip.Writer, srcPath, name string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	header.Method = zip.Deflate
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(writer, src)
	return err
}

func addDirToZip(zw *zip.Writer, rootDir, baseName string) error {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(rootAbs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(rootAbs, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		header, hErr := zip.FileInfoHeader(info)
		if hErr != nil {
			return hErr
		}
		header.Name = filepath.ToSlash(filepath.Join(baseName, rel))
		header.Method = zip.Deflate
		writer, wErr := zw.CreateHeader(header)
		if wErr != nil {
			return wErr
		}
		src, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer src.Close()
		_, copyErr := io.Copy(writer, src)
		return copyErr
	})
}

const (
	maxImportSize      = 256 << 20
	maxImportTotalSize = 512 << 20
)

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	if s.options.ContentDir == "" {
		writeError(w, http.StatusInternalServerError, "未配置站点路径，无法导入")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择要导入的 zip 文件")
		return
	}
	defer file.Close()

	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取上传文件失败: "+err.Error())
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "读取上传文件失败: "+err.Error())
		return
	}

	zr, err := zip.NewReader(file, size)
	if err != nil {
		writeError(w, http.StatusBadRequest, "不是有效的 zip 文件: "+err.Error())
		return
	}

	type plannedFile struct {
		dest  string
		entry *zip.File
	}
	var plans []plannedFile
	var totalSize uint64

	for _, entry := range zr.File {
		rawName := entry.Name
		if strings.HasSuffix(rawName, "/") || entry.FileInfo().IsDir() {
			continue
		}
		name, ok := safeZipName(rawName)
		if !ok {
			writeError(w, http.StatusBadRequest, "压缩包包含非法路径: "+rawName)
			return
		}
		if strings.HasPrefix(path.Base(name), ".") {
			continue
		}

		var dest string
		switch {
		case name == "daybook.yaml":
			if s.options.ConfigPath == "" {
				writeError(w, http.StatusBadRequest, "站点未配置 daybook.yaml 路径，无法恢复配置")
				return
			}
			dest = s.options.ConfigPath
		case name == "vault" || strings.HasPrefix(name, "vault/"):
			rel := strings.TrimPrefix(strings.TrimPrefix(name, "vault"), "/")
			if rel == "" {
				continue
			}
			dest = filepath.Join(s.options.ContentDir, filepath.FromSlash(rel))
			if !ensureWithinDir(s.options.ContentDir, dest) {
				writeError(w, http.StatusBadRequest, "压缩包包含越界路径: "+rawName)
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "压缩包包含未知内容: "+rawName)
			return
		}

		if strings.HasPrefix(filepath.Base(dest), ".") {
			continue
		}
		totalSize += entry.UncompressedSize64
		if totalSize > maxImportTotalSize {
			writeError(w, http.StatusBadRequest, "压缩包内容过大，已中止导入")
			return
		}
		plans = append(plans, plannedFile{dest: dest, entry: entry})
	}

	if len(plans) == 0 {
		writeError(w, http.StatusBadRequest, "压缩包中没有可导入的内容")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, plan := range plans {
		if err := extractZipEntry(plan.entry, plan.dest); err != nil {
			writeError(w, http.StatusInternalServerError, "写入文件失败: "+err.Error())
			return
		}
	}

	if s.options.Build != nil {
		if buildErr := s.options.Build(); buildErr != nil {
			writeError(w, http.StatusInternalServerError, "数据已导入，但重建站点失败: "+buildErr.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "files": len(plans)})
}

func safeZipName(name string) (string, bool) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", false
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", false
		}
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" {
		return "", false
	}
	return cleaned, true
}

func ensureWithinDir(root, target string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func extractZipEntry(entry *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
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
