package app

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Wenaixi/submatrix/save/method"
)

// ---------- 数据模型 ----------

// Share 是一个按组分发项，独立于组配置存在（可增删/开关）。
// 分享码即 URL 中的 code，12 字符 base62，不可枚举。
type Share struct {
	ID        string    `json:"id"`
	GroupName string    `json:"group"`
	Enabled   bool      `json:"enabled"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"` // 零值 = 永不过期
	Files     []string  `json:"files"`      // 空 = 全部文件
}

// IPHit 单次拉取记录（IP 脱敏后展示）。
type IPHit struct {
	IP   string    `json:"ip"`
	Time time.Time `json:"time"`
	File string    `json:"file"`
	UA   string    `json:"ua"`
}

// ShareStats 分享项的拉取统计。
type ShareStats struct {
	ShareID   string           `json:"share_id"`
	TotalHits int              `json:"total_hits"`
	UniqueIPs int              `json:"unique_ips"`
	LastHit   time.Time        `json:"last_hit"`
	PerFile   map[string]int   `json:"per_file"`
	IPHistory []IPHit          `json:"ip_history"` // 环形，上限 maxIPHistory
	seenIPs   map[string]struct{} // 内存态：唯一 IP 集合
}

// 常量
const (
	shareCodeLen   = 12
	maxIPHistory   = 500
	shareFileMax   = 4096 // 码生成最大尝试次数，防随机失败死循环
)

// ShareStore 管理分享项与统计的内存态 + 落盘。
type ShareStore struct {
	mu     sync.RWMutex
	shares map[string]*Share    // id → share
	stats  map[string]*ShareStats
	path   string // shares.json 路径
	statsPath string // share_stats.json 路径
	dirty  bool
}

// shareStore 全局单例（进程内唯一）。
var shareStore *ShareStore

// GetShareStore 返回全局分享存储（懒加载）。
func GetShareStore() *ShareStore {
	if shareStore == nil {
		shareStore = NewShareStore()
	}
	return shareStore
}

// NewShareStore 创建并加载分享存储。
func NewShareStore() *ShareStore {
	base := outputDir()
	ss := &ShareStore{
		shares:    make(map[string]*Share),
		stats:     make(map[string]*ShareStats),
		path:      filepath.Join(base, "shares.json"),
		statsPath: filepath.Join(base, "share_stats.json"),
	}
	ss.load()
	return ss
}

// outputDir 计算输出目录（与 save 包一致）。
func outputDir() string {
	if saver, err := method.NewLocalSaver(); err == nil {
		return saver.OutputPath
	}
	return "output"
}

// ---------- 分享项 CRUD ----------

// List 返回全部分享项（按创建时间倒序）。
func (s *ShareStore) List() []*Share {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Share, 0, len(s.shares))
	for _, sh := range s.shares {
		cp := *sh
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Get 按 id 取分享项。
func (s *ShareStore) Get(id string) (*Share, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sh, ok := s.shares[id]
	if !ok {
		return nil, false
	}
	cp := *sh
	return &cp, true
}

// Create 创建新分享；group 必须非空；返回生成的分享（含随机码）。
func (s *ShareStore) Create(group, note string, expiresAt time.Time, files []string) (*Share, error) {
	if group == "" {
		return nil, fmt.Errorf("分享必须指定组")
	}
	code, err := s.genShareCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sh := &Share{
		ID:        code,
		GroupName: group,
		Enabled:   true,
		Note:      note,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Files:     files,
	}
	s.mu.Lock()
	s.shares[code] = sh
	s.stats[code] = &ShareStats{
		ShareID: code,
		PerFile: make(map[string]int),
		seenIPs: make(map[string]struct{}),
	}
	s.dirty = true
	s.save() // 持锁态调用（save 已改为无锁内部版）
	s.mu.Unlock()
	slog.Info("分享已创建", "id", code, "组", group)
	return sh, nil
}

// Delete 删除分享项（统计数据保留在 stats 中并标记 deleted 由调用方处理）。
func (s *ShareStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.shares[id]; !ok {
		return false
	}
	delete(s.shares, id)
	s.dirty = true
	s.save()
	slog.Info("分享已删除", "id", id)
	return true
}

// Toggle 开关分享。
func (s *ShareStore) Toggle(id string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.shares[id]
	if !ok {
		return false
	}
	sh.Enabled = enabled
	s.dirty = true
	s.save()
	return true
}

// ---------- 统计 ----------

// RecordHit 记录一次拉取（内存累加，落盘由 Flush 或定时器触发）。
func (s *ShareStore) RecordHit(shareID, ip, file, ua string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.stats[shareID]
	if !ok {
		st = &ShareStats{ShareID: shareID, PerFile: make(map[string]int), seenIPs: make(map[string]struct{})}
		s.stats[shareID] = st
	}
	now := time.Now()
	st.TotalHits++
	st.LastHit = now
	st.PerFile[file]++
	if _, seen := st.seenIPs[ip]; !seen {
		st.seenIPs[ip] = struct{}{}
		st.UniqueIPs = len(st.seenIPs)
	}
	st.IPHistory = append(st.IPHistory, IPHit{IP: ip, Time: now, File: file, UA: ua})
	if len(st.IPHistory) > maxIPHistory {
		st.IPHistory = st.IPHistory[len(st.IPHistory)-maxIPHistory:]
	}
	s.dirty = true
}

// GetStats 取某分享的统计。
func (s *ShareStore) GetStats(shareID string) (*ShareStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.stats[shareID]
	if !ok {
		return nil, false
	}
	cp := *st
	cp.PerFile = make(map[string]int, len(st.PerFile))
	for k, v := range st.PerFile {
		cp.PerFile[k] = v
	}
	cp.IPHistory = append([]IPHit(nil), st.IPHistory...)
	return &cp, true
}

// ListStats 取全部统计（用于管理页）。
func (s *ShareStore) ListStats() map[string]*ShareStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*ShareStats, len(s.stats))
	for id, st := range s.stats {
		cp := *st
		cp.PerFile = make(map[string]int, len(st.PerFile))
		for k, v := range st.PerFile {
			cp.PerFile[k] = v
		}
		cp.IPHistory = append([]IPHit(nil), st.IPHistory...)
		out[id] = &cp
	}
	return out
}

// ---------- 持久化 ----------

// ensureDir 确保输出目录存在。
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return nil
}

// atomicWriteFile 原子写：临时文件 + rename，防写坏。
func atomicWriteFile(path string, data []byte) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写临时文件失败: %w", err)
	}
	return os.Rename(tmp, path)
}

// save 落盘 shares.json（仅 dirty 时）。调用方必须已持有 s.mu。
func (s *ShareStore) save() {
	if !s.dirty {
		return
	}
	list := make([]*Share, 0, len(s.shares))
	for _, sh := range s.shares {
		list = append(list, sh)
	}
	if data, err := json.MarshalIndent(list, "", "  "); err == nil {
		if err := atomicWriteFile(s.path, data); err != nil {
			slog.Error("保存分享配置失败", "err", err)
		} else {
			s.dirty = false
		}
	}
}

// saveStats 落盘 share_stats.json（独立频控，由 Flush 调用）。
func (s *ShareStore) saveStats() {
	list := make([]*ShareStats, 0, len(s.stats))
	for _, st := range s.stats {
		list = append(list, st)
	}
	if data, err := json.MarshalIndent(list, "", "  "); err == nil {
		if err := atomicWriteFile(s.statsPath, data); err != nil {
			slog.Error("保存分享统计失败", "err", err)
		}
	}
}

// Flush 强制把内存态落盘（定时器/退出时调用）。
func (s *ShareStore) Flush() {
	s.mu.Lock()
	s.dirty = true // 强制写 stats
	s.save()       // 持锁态调用
	s.saveStats()  // 持锁态调用
	s.mu.Unlock()
}

// load 从磁盘加载（shares.json + share_stats.json）。
func (s *ShareStore) load() {
	// 加载分享项
	if data, err := os.ReadFile(s.path); err == nil {
		var list []*Share
		if err := json.Unmarshal(data, &list); err == nil {
			for _, sh := range list {
				s.shares[sh.ID] = sh
			}
		}
	}
	// 加载统计
	if data, err := os.ReadFile(s.statsPath); err == nil {
		var list []*ShareStats
		if err := json.Unmarshal(data, &list); err == nil {
			for _, st := range list {
				ips := make(map[string]struct{})
				for _, h := range st.IPHistory {
					ips[h.IP] = struct{}{}
				}
				st.seenIPs = ips
				if st.UniqueIPs == 0 {
					st.UniqueIPs = len(ips)
				}
				s.stats[st.ShareID] = st
			}
		}
	}
}


// ---------- 工具 ----------

// genShareCode 生成 12 字符 base62 随机码（crypto/rand，不可枚举）。
func (s *ShareStore) genShareCode() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, shareCodeLen)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for attempt := 0; attempt < shareFileMax; attempt++ {
		raw := make([]byte, shareCodeLen)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("生成分享码失败: %w", err)
		}
		for i, b := range raw {
			buf[i] = alphabet[int(b)%len(alphabet)]
		}
		code := string(buf)
		if _, exists := s.shares[code]; !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("生成分享码碰撞次数过多")
}

// MaskIP 对 IP 末段打码（展示用）：1.2.3.4 → 1.2.3.*；IPv6 截断。
func MaskIP(ip string) string {
	// IPv6 取前 3 段
	for i, c := range ip {
		if c == ':' {
			if i > 0 {
				return ip[:i] + ":*"
			}
		}
	}
	// IPv4 末段打码
	lastDot := -1
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot > 0 {
		return ip[:lastDot] + ".*"
	}
	return "unknown"
}