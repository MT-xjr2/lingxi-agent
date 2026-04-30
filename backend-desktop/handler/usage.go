package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
	"lingxi-agent/usage"
)

// GetUsage GET /api/usage?range=today|7d|30d
func GetUsage(c *gin.Context) {
	rng := c.DefaultQuery("range", "7d")
	days := rangeToDays(rng)
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	summary, _ := db.SumUsageSince(since)
	byDay, _ := db.GroupUsageByDay(days)
	byModel, _ := db.GroupUsageByModel(days)
	recent, _ := db.ListRecentUsage(100)

	today := time.Now().Truncate(24 * time.Hour)
	todaySum, _ := db.SumUsageSince(today)

	c.JSON(http.StatusOK, gin.H{
		"range":    rng,
		"summary":  summary,
		"today":    todaySum,
		"by_day":   byDay,
		"by_model": byModel,
		"recent":   recent,
	})
}

func rangeToDays(r string) int {
	switch r {
	case "today", "1d":
		return 1
	case "7d":
		return 7
	case "30d":
		return 30
	case "90d":
		return 90
	default:
		return 7
	}
}

// GetUsageQuota GET /api/usage/quota?profile_id=...
// 调用上游 provider 的 usage 适配器，结果缓存 60s
func GetUsageQuota(c *gin.Context) {
	pidStr := c.Query("profile_id")
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil || pid <= 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	ap, err := db.GetAPIProfile(pid, false)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	prov, err := db.GetProvider(ap.ProviderID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	if snap, t, ok := db.GetUsageQuotaCache(pid); ok && time.Since(t) < 60*time.Second {
		c.Header("X-Cache", "hit")
		c.Data(http.StatusOK, "application/json", []byte(snap))
		return
	}

	actID, _, _, token := activeRuntimeSnapshot()
	if actID != pid || token == "" {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"reason":    "仅当所选档案为当前激活档案时可查询额度",
		})
		return
	}

	quota, err := usage.FetchQuota(prov.Code, prov.UsageAPIMeta, ap.BaseURL, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"available": false, "reason": err.Error()})
		return
	}
	bs, _ := json.Marshal(quota)
	db.SaveUsageQuotaSnapshot(pid, string(bs))
	c.Data(http.StatusOK, "application/json", bs)
}
