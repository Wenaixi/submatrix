package app

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Wenaixi/submatrix/config"
)

// ---------- 分享管理 API ----------

// listSharesHandler GET /api/shares
// 返回全部分享项 + 各自统计（IP 脱敏）。
func (app *App) listSharesHandler(c *gin.Context) {
	store := GetShareStore()
	shares := store.List()
	stats := store.ListStats()

	type shareResp struct {
		*Share
		TotalHits int               `json:"total_hits"`
		UniqueIPs int               `json:"unique_ips"`
		LastHit   time.Time         `json:"last_hit"`
		PerFile   map[string]int    `json:"per_file"`
		Link      string            `json:"link"`
	}

	out := make([]shareResp, 0, len(shares))
	for _, sh := range shares {
		resp := shareResp{Share: sh}
		if st, ok := stats[sh.ID]; ok {
			resp.TotalHits = st.TotalHits
			resp.UniqueIPs = st.UniqueIPs
			resp.LastHit = st.LastHit
			resp.PerFile = st.PerFile
		}
		resp.Link = buildShareLink(sh)
		out = append(out, resp)
	}
	c.JSON(http.StatusOK, gin.H{"shares": out})
}

// buildShareLink 构造分享首页链接（含公网域名或本机）。
func buildShareLink(sh *Share) string {
	base := config.GlobalConfig.PublicURL
	if base == "" {
		return "/share/" + sh.ID
	}
	return trimRightSlash(base) + "/share/" + sh.ID
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// createShareHandler POST /api/shares
// body: {group, note, expires_at(rfc3339 或空=永不过期), files(空=全部)}
func (app *App) createShareHandler(c *gin.Context) {
	var req struct {
		Group     string    `json:"group"`
		Note      string    `json:"note"`
		ExpiresAt time.Time `json:"expires_at"`
		Files     []string  `json:"files"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if req.Group == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须指定分享的组"})
		return
	}
	// 校验组存在
	groupExists := false
	for _, g := range config.GlobalConfig.Groups {
		if g.Name == req.Group {
			groupExists = true
			break
		}
	}
	if !groupExists {
		// 默认组（无 groups 配置时）也允许分享
		if len(config.GlobalConfig.Groups) == 0 {
			groupExists = true
		}
	}
	if !groupExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "组不存在: " + req.Group})
		return
	}

	sh, err := GetShareStore().Create(req.Group, req.Note, req.ExpiresAt, req.Files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"share": sh, "link": buildShareLink(sh)})
}

// toggleShareHandler POST /api/shares/toggle
// body: {id, enabled}
func (app *App) toggleShareHandler(c *gin.Context) {
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if !GetShareStore().Toggle(req.ID, req.Enabled) {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在: " + req.ID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteShareHandler POST /api/shares/delete
// body: {id}
func (app *App) deleteShareHandler(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if !GetShareStore().Delete(req.ID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在: " + req.ID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// shareStatsHandler GET /api/shares/stats?code=<id>
// 返回某分享的统计与 IP 历史（IP 脱敏）。
func (app *App) shareStatsHandler(c *gin.Context) {
	id := c.Query("code")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 code 参数"})
		return
	}
	st, ok := GetShareStore().GetStats(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "统计不存在: " + id})
		return
	}

	// IP 历史脱敏
	type ipView struct {
		IP   string    `json:"ip"`
		Time time.Time `json:"time"`
		File string    `json:"file"`
		UA   string    `json:"ua"`
	}
	history := make([]ipView, 0, len(st.IPHistory))
	for _, h := range st.IPHistory {
		history = append(history, ipView{IP: MaskIP(h.IP), Time: h.Time, File: h.File, UA: h.UA})
	}

	c.JSON(http.StatusOK, gin.H{
		"share_id":   st.ShareID,
		"total_hits": st.TotalHits,
		"unique_ips": st.UniqueIPs,
		"last_hit":   st.LastHit,
		"per_file":   st.PerFile,
		"ip_history": history,
	})
}

// ---------- 工具：组产物文件清单 ----------

// listGroupFiles 返回某组产物目录下实际存在的文件。
func listGroupFiles(groupName string) []string {
	dir := filepath.Join(outputDir(), "groups", groupName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && shareAllowedFiles[e.Name()] {
			out = append(out, e.Name())
		}
	}
	return out
}