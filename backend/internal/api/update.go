// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import "github.com/gin-gonic/gin"

// handleUpdateCheck 替前端去问 GitHub。登录即可，升级命令只在前端对管理员展示。
func (s *Server) handleUpdateCheck(c *gin.Context) {
	if s.updater == nil {
		ok(c, gin.H{
			"current":   s.version,
			"latest":    "",
			"hasUpdate": false,
			"channel":   "dev",
			"status":    "dev",
			"checkedAt": 0,
			"inDocker":  false,
		})
		return
	}
	ok(c, s.updater.Check())
}
