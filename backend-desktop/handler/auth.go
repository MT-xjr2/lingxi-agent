package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lingxi-agent/db"

	"github.com/gin-gonic/gin"
)

// ─── 用户状态查询 ────────────────────────────────────────────────

// GetCurrentUser GET /api/auth/me — 查询当前用户，无用户返回 null
func GetCurrentUser(c *gin.Context) {
	u, err := db.GetCurrentUser()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"user": nil, "logged_in": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "logged_in": true})
}

// HasUser GET /api/auth/status — 是否已有任何用户
func HasUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"has_user": db.HasAnyUser()})
}

// ─── 游客登录 ────────────────────────────────────────────────────

// GuestLogin POST /api/auth/guest — 创建游客身份
func GuestLogin(c *gin.Context) {
	var body struct {
		Nickname string `json:"nickname"`
	}
	c.ShouldBindJSON(&body)

	nickname := body.Nickname
	if nickname == "" {
		nickname = "游客"
	}

	guestID := generateGuestID()

	u := &db.User{
		Provider:   "guest",
		ProviderID: guestID,
		Nickname:   nickname,
		AvatarURL:  "",
		Email:      "",
	}
	id, err := db.CreateUser(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	u.ID = id
	c.JSON(http.StatusOK, gin.H{"user": u})
}

// ─── OAuth 回调处理 ──────────────────────────────────────────────

// OAuthCallback POST /api/auth/oauth/callback — Electron 拿到 auth code 后调用
func OAuthCallback(c *gin.Context) {
	var body struct {
		Provider    string `json:"provider"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Provider == "" || body.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider and code are required"})
		return
	}

	oauthCfg, err := db.GetOAuthConfig(body.Provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth config not found for provider: " + body.Provider})
		return
	}

	var userInfo *oauthUserInfo
	switch body.Provider {
	case "google":
		userInfo, err = exchangeGoogle(oauthCfg, body.Code, body.RedirectURI)
	case "wechat":
		userInfo, err = exchangeWechat(oauthCfg, body.Code)
	case "qq":
		userInfo, err = exchangeQQ(oauthCfg, body.Code, body.RedirectURI)
	case "dingtalk":
		userInfo, err = exchangeDingtalk(oauthCfg, body.Code)
	case "douyin":
		userInfo, err = exchangeDouyin(oauthCfg, body.Code)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider: " + body.Provider})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "OAuth exchange failed: " + err.Error()})
		return
	}

	existing, _ := db.GetUserByProviderID(body.Provider, userInfo.ID)
	if existing != nil {
		db.UpdateUser(existing.ID, userInfo.Nickname, userInfo.AvatarURL, userInfo.Email)
		existing.Nickname = userInfo.Nickname
		existing.AvatarURL = userInfo.AvatarURL
		existing.Email = userInfo.Email
		c.JSON(http.StatusOK, gin.H{"user": existing})
		return
	}

	u := &db.User{
		Provider:   body.Provider,
		ProviderID: userInfo.ID,
		Nickname:   userInfo.Nickname,
		AvatarURL:  userInfo.AvatarURL,
		Email:      userInfo.Email,
	}
	id, err := db.CreateUser(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	u.ID = id
	c.JSON(http.StatusOK, gin.H{"user": u})
}

// ─── 登出 ────────────────────────────────────────────────────────

// Logout POST /api/auth/logout — 清除所有用户记录
func Logout(c *gin.Context) {
	db.DeleteAllUsers()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── OAuth Config 管理 ──────────────────────────────────────────

// ListOAuthConfigs GET /api/auth/oauth-configs
func ListOAuthConfigs(c *gin.Context) {
	configs, err := db.ListOAuthConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	safe := make([]map[string]interface{}, 0, len(configs))
	for _, cfg := range configs {
		safe = append(safe, map[string]interface{}{
			"provider": cfg.Provider,
			"app_id":   cfg.AppID,
			"has_secret": cfg.AppSecret != "",
		})
	}
	c.JSON(http.StatusOK, safe)
}

// SaveOAuthConfig POST /api/auth/oauth-configs
func SaveOAuthConfig(c *gin.Context) {
	var body db.OAuthConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.UpsertOAuthConfig(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── OAuth 交换实现 ──────────────────────────────────────────────

type oauthUserInfo struct {
	ID        string
	Nickname  string
	AvatarURL string
	Email     string
}

var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

func exchangeGoogle(cfg *db.OAuthConfig, code, redirectURI string) (*oauthUserInfo, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {cfg.AppID},
		"client_secret": {cfg.AppSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	resp, err := oauthHTTPClient.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	json.Unmarshal(body, &tokenResp)
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response")
	}

	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	uResp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()
	uBody, _ := io.ReadAll(uResp.Body)

	var gUser struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	json.Unmarshal(uBody, &gUser)
	return &oauthUserInfo{
		ID:        gUser.ID,
		Nickname:  gUser.Name,
		AvatarURL: gUser.Picture,
		Email:     gUser.Email,
	}, nil
}

func exchangeWechat(cfg *db.OAuthConfig, code string) (*oauthUserInfo, error) {
	tokenURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		cfg.AppID, cfg.AppSecret, code)

	resp, err := oauthHTTPClient.Get(tokenURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	json.Unmarshal(body, &tokenResp)
	if tokenResp.ErrCode != 0 {
		return nil, fmt.Errorf("wechat token: %s", tokenResp.ErrMsg)
	}

	userURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s",
		tokenResp.AccessToken, tokenResp.OpenID)
	uResp, err := oauthHTTPClient.Get(userURL)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()
	uBody, _ := io.ReadAll(uResp.Body)

	var wxUser struct {
		OpenID    string `json:"openid"`
		Nickname  string `json:"nickname"`
		HeadImgUR string `json:"headimgurl"`
	}
	json.Unmarshal(uBody, &wxUser)
	return &oauthUserInfo{
		ID:        wxUser.OpenID,
		Nickname:  wxUser.Nickname,
		AvatarURL: wxUser.HeadImgUR,
	}, nil
}

func exchangeQQ(cfg *db.OAuthConfig, code, redirectURI string) (*oauthUserInfo, error) {
	tokenURL := fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/token?grant_type=authorization_code&client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&fmt=json",
		cfg.AppID, cfg.AppSecret, code, url.QueryEscape(redirectURI))

	resp, err := oauthHTTPClient.Get(tokenURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	json.Unmarshal(body, &tokenResp)
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("qq token failed: %s", string(body))
	}

	meResp, err := oauthHTTPClient.Get("https://graph.qq.com/oauth2.0/me?access_token=" + tokenResp.AccessToken + "&fmt=json")
	if err != nil {
		return nil, err
	}
	defer meResp.Body.Close()
	meBody, _ := io.ReadAll(meResp.Body)
	var me struct {
		OpenID string `json:"openid"`
	}
	json.Unmarshal(meBody, &me)

	infoURL := fmt.Sprintf(
		"https://graph.qq.com/user/get_user_info?access_token=%s&oauth_consumer_key=%s&openid=%s",
		tokenResp.AccessToken, cfg.AppID, me.OpenID)
	infoResp, err := oauthHTTPClient.Get(infoURL)
	if err != nil {
		return nil, err
	}
	defer infoResp.Body.Close()
	infoBody, _ := io.ReadAll(infoResp.Body)
	var qqUser struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"figureurl_qq_2"`
	}
	json.Unmarshal(infoBody, &qqUser)
	return &oauthUserInfo{
		ID:        me.OpenID,
		Nickname:  qqUser.Nickname,
		AvatarURL: qqUser.Avatar,
	}, nil
}

func exchangeDingtalk(cfg *db.OAuthConfig, code string) (*oauthUserInfo, error) {
	payload := map[string]string{
		"clientId":     cfg.AppID,
		"clientSecret": cfg.AppSecret,
		"code":         code,
		"grantType":    "authorization_code",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.dingtalk.com/v1.0/oauth2/userAccessToken", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken string `json:"accessToken"`
	}
	json.Unmarshal(body, &tokenResp)
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("dingtalk token failed: %s", string(body))
	}

	req2, _ := http.NewRequest("GET", "https://api.dingtalk.com/v1.0/contact/users/me", nil)
	req2.Header.Set("x-acs-dingtalk-access-token", tokenResp.AccessToken)
	uResp, err := oauthHTTPClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()
	uBody, _ := io.ReadAll(uResp.Body)

	var dtUser struct {
		OpenID    string `json:"openId"`
		Nick      string `json:"nick"`
		AvatarURL string `json:"avatarUrl"`
		Email     string `json:"email"`
	}
	json.Unmarshal(uBody, &dtUser)
	return &oauthUserInfo{
		ID:        dtUser.OpenID,
		Nickname:  dtUser.Nick,
		AvatarURL: dtUser.AvatarURL,
		Email:     dtUser.Email,
	}, nil
}

func exchangeDouyin(cfg *db.OAuthConfig, code string) (*oauthUserInfo, error) {
	data := url.Values{
		"client_key":    {cfg.AppID},
		"client_secret": {cfg.AppSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	resp, err := oauthHTTPClient.PostForm("https://open.douyin.com/oauth/access_token/", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var wrapper struct {
		Data struct {
			AccessToken string `json:"access_token"`
			OpenID      string `json:"open_id"`
			ErrNo       int    `json:"error_code"`
		} `json:"data"`
	}
	json.Unmarshal(body, &wrapper)
	if wrapper.Data.AccessToken == "" {
		return nil, fmt.Errorf("douyin token failed: %s", string(body))
	}

	req, _ := http.NewRequest("GET",
		"https://open.douyin.com/oauth/userinfo/?access_token="+wrapper.Data.AccessToken+"&open_id="+wrapper.Data.OpenID, nil)
	uResp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()
	uBody, _ := io.ReadAll(uResp.Body)

	var dyWrapper struct {
		Data struct {
			OpenID   string `json:"open_id"`
			Nickname string `json:"nickname"`
			Avatar   string `json:"avatar"`
		} `json:"data"`
	}
	json.Unmarshal(uBody, &dyWrapper)
	return &oauthUserInfo{
		ID:        dyWrapper.Data.OpenID,
		Nickname:  dyWrapper.Data.Nickname,
		AvatarURL: dyWrapper.Data.Avatar,
	}, nil
}

// ─── 工具函数 ────────────────────────────────────────────────────

func generateGuestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "guest_" + hex.EncodeToString(b)
}
