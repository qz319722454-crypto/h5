package handlers

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"h5-backend/models"
	"net/http"
	"strconv"
	"sync"
	"os"
	"path/filepath"
	"io"
	"github.com/google/uuid"
	"time"
	"log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 设置 ping/pong 超时
	HandshakeTimeout: 10 * time.Second,
}

var (
	connections = make(map[uint]*websocket.Conn)
	connMutex   sync.Mutex
)

// SetupChatRoutes sets up routes for chat operations
func SetupChatRoutes(r *gin.Engine, db *gorm.DB) {
	chat := r.Group("/chat")
	{
		chat.GET("/ws/:csId", func(c *gin.Context) { wsHandler(c, db) })
		chat.POST("/send", func(c *gin.Context) { sendUserMessage(c, db) })
		chat.POST("/subscribe", func(c *gin.Context) { subscribeHandler(c, db) })
		chat.POST("/login", func(c *gin.Context) { loginHandler(c, db) })
		chat.POST("/upload", func(c *gin.Context) { uploadImage(c, db) })
		chat.GET("/history", func(c *gin.Context) { getChatHistory(c, db) })
		chat.GET("/cs/:csId/user/:userId/messages", func(c *gin.Context) { getCSUserMessages(c, db) })
		chat.POST("/cs/send", func(c *gin.Context) { sendCSMessage(c, db) })
		chat.GET("/cs/:csId/qrcode", func(c *gin.Context) { getCSQRCode(c, db) })
		chat.POST("/heartbeat", func(c *gin.Context) { userHeartbeat(c, db) })
		chat.DELETE("/message/:id", func(c *gin.Context) { deleteMessage(c, db) })
		chat.POST("/message/:id/read", func(c *gin.Context) { markMessageAsRead(c, db) })
		chat.POST("/cs/:csId/user/:userId/push", func(c *gin.Context) { manualPushNotification(c, db) })
		chat.GET("/cs/:csId/user/:userId/push-status", func(c *gin.Context) { checkPushStatus(c, db) })
	}
}

func wsHandler(c *gin.Context, db *gorm.DB) {
	csId := c.Param("csId")
	id := parseUint(csId) // Helper to parse uint
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// 设置读写超时
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	
	// 设置 ping/pong 处理器
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	connMutex.Lock()
	connections[id] = conn
	connMutex.Unlock()

	defer func() {
		connMutex.Lock()
		delete(connections, id)
		connMutex.Unlock()
		conn.Close()
	}()

	// 启动心跳 goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// 读取消息
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			close(done)
			break
		}
		
		// 处理 ping 消息
		if messageType == websocket.PingMessage {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			conn.WriteMessage(websocket.PongMessage, nil)
			continue
		}
		
		// 处理文本消息
		if messageType == websocket.TextMessage {
			// Handle CS reply: parse message, save, send push to user
			var msg models.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}
			msg.FromUser = false
			msg.CustomerServiceID = id
			msg.IsImage = msg.ImageURL != ""
			db.Create(&msg)
			if msg.IsImage {
				sendSubscriptionPush(db, msg.UserID, id, "您收到一张图片")
			} else {
				sendSubscriptionPush(db, msg.UserID, id, msg.Content)
			}
		}
	}
}

func sendUserMessage(c *gin.Context, db *gorm.DB) {
	var req struct {
		AppID    string `json:"appId"`
		OpenID   string `json:"openId"`
		Content  string `json:"content"`
		ImageURL string `json:"imageUrl"` // Optional
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证必须包含appId
	if req.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供小程序 AppID"})
		return
	}

	// Find or create user
	var user models.User
	miniAppID := findMiniAppID(db, req.AppID)
	if miniAppID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该小程序"})
		return
	}
	
	// 检查用户是否是新用户（首次发送消息）
	isNewUser := false
	if err := db.Where("open_id = ?", req.OpenID).First(&user).Error; err != nil {
		// 用户不存在，创建新用户
		user = models.User{OpenID: req.OpenID, MiniAppID: miniAppID}
		db.Create(&user)
		isNewUser = true
	} else {
		// 检查用户是否发送过消息
		var msgCount int64
		db.Model(&models.Message{}).Where("user_id = ? AND from_user = ?", user.ID, true).Count(&msgCount)
		isNewUser = (msgCount == 0)
	}

	// Find assigned CS by appID
	csID := findAssignedCS(db, req.AppID)
	if csID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "该小程序未分配客服"})
		return
	}

	// 更新用户最后活动时间
	now := time.Now()
	user.LastActiveTime = &now
	db.Save(&user)

	msg := models.Message{
		UserID:            user.ID,
		CustomerServiceID: csID,
		Content:           req.Content,
		FromUser:          true,
		IsImage:           req.ImageURL != "",
		ImageURL:          req.ImageURL,
	}
	db.Create(&msg)

	// 如果是新用户且设置了欢迎语，发送欢迎语
	if isNewUser {
		var cs models.CustomerService
		if err := db.First(&cs, csID).Error; err == nil && cs.WelcomeMessage != "" {
			// 发送欢迎语
			welcomeMsg := models.Message{
				UserID:            user.ID,
				CustomerServiceID: csID,
				Content:           cs.WelcomeMessage,
				FromUser:          false,
				IsImage:           false,
			}
			db.Create(&welcomeMsg)
			
			// 通过WebSocket发送给客服（如果连接）
			connMutex.Lock()
			conn, ok := connections[csID]
			connMutex.Unlock()
			if ok {
				conn.WriteJSON(welcomeMsg)
			}
			
			// 发送订阅推送
			sendSubscriptionPush(db, user.ID, csID, cs.WelcomeMessage)
		}
	}

	// Send to CS if connected
	connMutex.Lock()
	conn, ok := connections[csID]
	connMutex.Unlock()
	if ok {
		conn.WriteJSON(msg) // Send full msg including ImageURL
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

func subscribeHandler(c *gin.Context, db *gorm.DB) {
	var req struct {
		OpenID string `json:"openId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	var user models.User
	if err := db.Where("open_id = ?", req.OpenID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	user.Subscribed = true
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"status": "subscribed"})
}

func loginHandler(c *gin.Context, db *gorm.DB) {
	var req struct {
		Code   string `json:"code"`
		AppID  string `json:"appId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.Code == "" || req.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code 和 appId 不能为空"})
		return
	}
	// Get miniapp secret
	var ma models.MiniApp
	if err := db.Where("app_id = ?", req.AppID).First(&ma).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该小程序"})
		return
	}

	// Exchange code for openid
	url := "https://api.weixin.qq.com/sns/jscode2session?appid=" + req.AppID + "&secret=" + ma.Secret + "&js_code=" + req.Code + "&grant_type=authorization_code"
	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "微信API调用失败"})
		return
	}
	defer resp.Body.Close()
	var result struct {
		OpenID string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	
	if result.ErrCode != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "微信登录失败: " + result.ErrMsg})
		return
	}
	
	if result.OpenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取用户信息失败"})
		return
	}

	// Save or update user
	var user models.User
	db.Where("open_id = ?", result.OpenID).FirstOrCreate(&user, models.User{OpenID: result.OpenID, MiniAppID: ma.ID})
	
	// 更新用户的 mini_app_id（如果小程序ID变化）
	if user.MiniAppID != ma.ID {
		user.MiniAppID = ma.ID
		db.Save(&user)
	}

	c.JSON(http.StatusOK, gin.H{
		"openId": result.OpenID,
		"templateId": ma.TemplateID, // 返回模板ID供小程序使用
		"subscribed": user.Subscribed, // 返回订阅状态
	})
}

// New upload handler
func uploadImage(c *gin.Context, db *gorm.DB) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择图片文件"})
		return
	}
	// Save file to uploads dir
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, os.ModePerm)
	filename := uuid.New().String() + filepath.Ext(file.Filename)
	dst := filepath.Join(uploadDir, filename)
	out, err := os.Create(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer out.Close()
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开文件失败"})
		return
	}
	defer src.Close()
	io.Copy(out, src)
	// Return URL (assume served from /uploads)
	url := "https://kefu.chacaitx.cn/uploads/" + filename
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func getChatHistory(c *gin.Context, db *gorm.DB) {
	openID := c.Query("openId")
	appID := c.Query("appId")
	if openID == "" || appID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数"})
		return
	}
	var user models.User
	if err := db.Where("open_id = ? AND mini_app_id = (SELECT id FROM mini_apps WHERE app_id = ?)", openID, appID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var messages []models.Message
	db.Where("user_id = ? AND is_deleted = ?", user.ID, false).Order("created_at ASC").Find(&messages)
	
	// 标记所有客服发送的消息为已读
	db.Model(&models.Message{}).
		Where("user_id = ? AND from_user = ? AND user_read = ?", user.ID, false, false).
		Update("user_read", true)
	
	c.JSON(http.StatusOK, messages)
}

// getCSUserMessages 获取客服与指定用户的聊天记录
func getCSUserMessages(c *gin.Context, db *gorm.DB) {
	csID := parseUint(c.Param("csId"))
	userID := parseUint(c.Param("userId"))
	
	if csID == 0 || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	
	// 验证用户是否属于分配给该客服的小程序
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 检查该用户的小程序是否分配给该客服
	var assignment models.Assignment
	if err := db.Where("mini_app_id = ? AND customer_service_id = ?", user.MiniAppID, csID).First(&assignment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该用户不属于您负责的小程序"})
		return
	}
	
	var messages []models.Message
	db.Where("user_id = ? AND customer_service_id = ? AND is_deleted = ?", userID, csID, false).
		Order("created_at ASC").Find(&messages)
	
	// 标记所有用户发送的消息为已读
	db.Model(&models.Message{}).
		Where("user_id = ? AND customer_service_id = ? AND from_user = ? AND is_read = ?", userID, csID, true, false).
		Update("is_read", true)
	
	c.JSON(http.StatusOK, messages)
}

// sendCSMessage 客服发送消息
func sendCSMessage(c *gin.Context, db *gorm.DB) {
	var req struct {
		UserID            uint   `json:"UserID"`
		CustomerServiceID uint   `json:"CustomerServiceID"`
		Content           string `json:"Content"`
		ImageURL          string `json:"ImageURL"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	
	// 验证用户是否属于分配给该客服的小程序
	var user models.User
	if err := db.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 检查该用户的小程序是否分配给该客服
	var assignment models.Assignment
	if err := db.Where("mini_app_id = ? AND customer_service_id = ?", user.MiniAppID, req.CustomerServiceID).First(&assignment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该用户不属于您负责的小程序"})
		return
	}
	
	msg := models.Message{
		UserID:            req.UserID,
		CustomerServiceID: req.CustomerServiceID,
		Content:           req.Content,
		FromUser:          false,
		IsImage:           req.ImageURL != "",
		ImageURL:          req.ImageURL,
	}
	
	if err := db.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 发送订阅推送（每条消息都尝试推送，sendSubscriptionPush 内部会检查订阅状态）
	if req.ImageURL != "" {
		sendSubscriptionPush(db, req.UserID, req.CustomerServiceID, "您收到一张图片")
	} else {
		sendSubscriptionPush(db, req.UserID, req.CustomerServiceID, req.Content)
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "sent", "message": msg})
}

// deleteMessage 删除消息
func deleteMessage(c *gin.Context, db *gorm.DB) {
	messageID := parseUint(c.Param("id"))
	if messageID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息ID无效"})
		return
	}
	
	var msg models.Message
	if err := db.First(&msg, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "消息不存在"})
		return
	}
	
	// 软删除：标记为已删除
	msg.IsDeleted = true
	db.Save(&msg)
	
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// markMessageAsRead 标记消息为已读（用户端）
func markMessageAsRead(c *gin.Context, db *gorm.DB) {
	messageID := parseUint(c.Param("id"))
	if messageID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息ID无效"})
		return
	}
	
	var msg models.Message
	if err := db.First(&msg, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "消息不存在"})
		return
	}
	
	// 只标记客服发送的消息为已读
	if !msg.FromUser {
		msg.UserRead = true
		db.Save(&msg)
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "read"})
}

// Helpers (implement properly)
func parseUint(s string) uint {
	u, _ := strconv.ParseUint(s, 10, 32)
	return uint(u)
}
func findMiniAppID(db *gorm.DB, appID string) uint {
	var ma models.MiniApp
	db.Where("app_id = ?", appID).First(&ma)
	return ma.ID
}
func findAssignedCS(db *gorm.DB, appID string) uint {
	var assign models.Assignment
	db.Joins("JOIN mini_apps ON assignments.mini_app_id = mini_apps.id").
		Where("mini_apps.app_id = ?", appID).
		First(&assign)
	return assign.CustomerServiceID
}
func sendSubscriptionPush(db *gorm.DB, userID uint, csID uint, content string) {
	// 使用 goroutine 异步推送，但添加错误处理和日志
	go func() {
		log.Printf("[推送] 开始推送，userID=%d, csID=%d, content=%s", userID, csID, content)
		
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			// 用户不存在，不推送
			log.Printf("[推送] ❌ 用户不存在，userID=%d, error=%v", userID, err)
			return
		}
		log.Printf("[推送] ✓ 用户存在，openID=%s", user.OpenID)
		
		// 检查用户是否已订阅，如果未订阅则不推送
		if !user.Subscribed {
			log.Printf("[推送] ❌ 用户未订阅，userID=%d, openID=%s, Subscribed=%v", userID, user.OpenID, user.Subscribed)
			return
		}
		log.Printf("[推送] ✓ 用户已订阅")

		// 检查用户是否在线（1分钟内有活动，实时检测）
		isOnline := false
		if user.LastActiveTime != nil {
			timeSinceActive := time.Since(*user.LastActiveTime)
			isOnline = timeSinceActive < 1*time.Minute
		}
		
		if isOnline {
			log.Printf("[推送] ⏭️  用户在线，跳过推送，userID=%d, 最后活动时间: %v", userID, user.LastActiveTime)
			return
		}
		log.Printf("[推送] ✓ 用户不在线，继续推送")

		// 获取客服名称
		var cs models.CustomerService
		csName := "客服"
		if err := db.First(&cs, csID).Error; err == nil {
			csName = cs.Name
		}
		log.Printf("[推送] ✓ 客服名称: %s", csName)

		var ma models.MiniApp
		if err := db.First(&ma, user.MiniAppID).Error; err != nil {
			// 小程序不存在，不推送
			log.Printf("[推送] ❌ 小程序不存在，userID=%d, miniAppID=%d, error=%v", userID, user.MiniAppID, err)
			return
		}
		log.Printf("[推送] ✓ 小程序存在，appID=%s", ma.AppID)
		
		// 检查模板ID是否存在
		if ma.TemplateID == "" {
			// 模板ID未配置，不推送
			log.Printf("[推送] ❌ 模板ID未配置，userID=%d, appID=%s", userID, ma.AppID)
			return
		}
		log.Printf("[推送] ✓ 模板ID已配置，templateID=%s", ma.TemplateID)

		// Get access_token
		tokenURL := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + ma.AppID + "&secret=" + ma.Secret
		log.Printf("[推送] 正在获取 access_token...")
		resp, err := http.Get(tokenURL)
		if err != nil {
			log.Printf("[推送] ❌ 获取 access_token 请求失败，userID=%d, error=%v", userID, err)
			return
		}
		defer resp.Body.Close()
		
		var tokenResp struct {
			AccessToken string `json:"access_token"`
			ErrCode     int    `json:"errcode"`
			ErrMsg      string `json:"errmsg"`
		}
		json.NewDecoder(resp.Body).Decode(&tokenResp)
		
		if tokenResp.ErrCode != 0 || tokenResp.AccessToken == "" {
			log.Printf("[推送] ❌ 获取 access_token 失败，userID=%d, errCode=%d, errMsg=%s", userID, tokenResp.ErrCode, tokenResp.ErrMsg)
			return
		}
		log.Printf("[推送] ✓ 获取 access_token 成功")

		// 格式化消息内容
		messageContent := "您收到新的消息,请点击查看!"
		if content != "" && content != "您收到一张图片" {
			// 限制内容长度（微信订阅消息 thing 类型最多 20 个字符）
			if len([]rune(content)) > 20 {
				messageContent = string([]rune(content)[:20])
			} else {
				messageContent = content
			}
		}
		log.Printf("[推送] 推送内容: %s", messageContent)

		// 获取当前时间并格式化
		now := time.Now()
		timeStr := now.Format("2006-01-02 15:04:05")
		log.Printf("[推送] 发送时间: %s", timeStr)

		// Send subscription message - 按照模板格式发送
		// 根据错误信息，模板需要 time2 字段，不是 time3
		sendURL := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + tokenResp.AccessToken
		data := map[string]interface{}{
			"touser":           user.OpenID,
			"template_id":      ma.TemplateID,
			"page":             "pages/index/index?p=true", // 跳转到客服页面
			"miniprogram_state": "formal", // 正式版小程序
			"lang":             "zh_CN",   // 语言
			"data": map[string]map[string]string{
				"name1": {"value": csName},                    // 发送者名称
				"thing2": {"value": messageContent},          // 消息内容
				"time2": {"value": timeStr},                  // 发送时间（模板需要 time2）
			},
		}
		jsonData, _ := json.Marshal(data)
		log.Printf("[推送] 推送数据: %s", string(jsonData))
		
		// 发送推送并检查响应
		log.Printf("[推送] 正在发送推送请求...")
		pushResp, err := http.Post(sendURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[推送] ❌ 发送推送请求失败，userID=%d, error=%v", userID, err)
			return
		}
		defer pushResp.Body.Close()
		
		// 读取响应内容
		bodyBytes, _ := io.ReadAll(pushResp.Body)
		log.Printf("[推送] 推送响应状态码: %d, 响应内容: %s", pushResp.StatusCode, string(bodyBytes))
		
		var pushResult struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		json.Unmarshal(bodyBytes, &pushResult)
		
		if pushResult.ErrCode == 0 {
			log.Printf("[推送] ✅ 推送成功！userID=%d, openID=%s, content=%s", userID, user.OpenID, messageContent)
			// 推送成功，订阅关系仍然有效，不需要更新订阅状态
		} else {
			log.Printf("[推送] ❌ 推送失败，userID=%d, errCode=%d, errMsg=%s", userID, pushResult.ErrCode, pushResult.ErrMsg)
			// 常见错误码说明
			needResubscribe := false
			if pushResult.ErrCode == 43101 {
				log.Printf("[推送] ⚠️  错误码43101: 用户拒绝接受消息，该用户不能再给此公众号下发消息，需要重新订阅")
				needResubscribe = true
			} else if pushResult.ErrCode == 47003 {
				log.Printf("[推送] ⚠️  错误码47003: 参数错误，可能是模板参数格式不正确")
				log.Printf("[推送] ⚠️  请检查模板字段名称是否正确，当前使用的字段: name1(发送者), thing2(内容), time2(时间)")
				log.Printf("[推送] ⚠️  错误详情: %s", pushResult.ErrMsg)
			} else if pushResult.ErrCode == 40037 {
				log.Printf("[推送] ⚠️  错误码40037: 模板ID不正确")
			} else if pushResult.ErrCode == 40001 {
				log.Printf("[推送] ⚠️  错误码40001: access_token无效，需要重新获取")
			} else if pushResult.ErrCode == 40013 {
				log.Printf("[推送] ⚠️  错误码40013: 不合法的AppID")
			} else if pushResult.ErrCode == 45009 {
				log.Printf("[推送] ⚠️  错误码45009: 接口调用超过限制（频率限制）")
				log.Printf("[推送] ⚠️  注意：频率限制不会导致订阅失效，只是暂时无法发送，稍后可以重试")
				// 频率限制不会导致订阅失效，所以不设置 needResubscribe
			} else if pushResult.ErrCode == 20001 {
				log.Printf("[推送] ⚠️  错误码20001: 系统繁忙，请稍后再试")
			} else if pushResult.ErrCode == 43104 {
				log.Printf("[推送] ⚠️  错误码43104: 订阅关系已失效，需要重新订阅")
				log.Printf("[推送] ⚠️  可能原因：1) 使用了一次性订阅消息模板（发送一次后失效）")
				log.Printf("[推送] ⚠️  可能原因：2) 订阅关系过期（长时间未使用）")
				log.Printf("[推送] ⚠️  建议：检查模板类型，如果是客服场景，应使用长期订阅消息模板")
				needResubscribe = true
			} else {
				log.Printf("[推送] ⚠️  未知错误码: %d, 错误信息: %s", pushResult.ErrCode, pushResult.ErrMsg)
			}
			
			// 只有在明确需要重新订阅的情况下（43101用户拒绝、43104订阅失效）才更新订阅状态为false
			// 其他所有错误（频率限制、系统繁忙、参数错误等）都不会导致订阅失效，保持 subscribed = true
			if needResubscribe {
				log.Printf("[推送] 🔄 标记用户需要重新订阅，userID=%d", userID)
				db.Model(&user).Update("subscribed", false)
			} else {
				log.Printf("[推送] ℹ️  订阅关系仍然有效（subscribed=true），只是本次推送失败，userID=%d", userID)
				// 确保订阅状态保持为true（防止之前被错误设置为false）
				if !user.Subscribed {
					log.Printf("[推送] 🔧 修复订阅状态：将 subscribed 从 false 恢复为 true，userID=%d", userID)
					db.Model(&user).Update("subscribed", true)
				}
			}
		}
	}()
}

// manualPushNotification 手动推送订阅消息（客服端触发）
func manualPushNotification(c *gin.Context, db *gorm.DB) {
	csID := parseUint(c.Param("csId"))
	userID := parseUint(c.Param("userId"))
	
	if csID == 0 || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	
	// 验证用户是否存在
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 检查该用户的小程序是否分配给该客服
	var assignment models.Assignment
	if err := db.Where("mini_app_id = ? AND customer_service_id = ?", user.MiniAppID, csID).First(&assignment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该用户不属于您负责的小程序"})
		return
	}
	
	// 检查用户是否已订阅
	if !user.Subscribed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户未授权订阅消息"})
		return
	}
	
	// 直接推送订阅消息
	sendSubscriptionPush(db, userID, csID, "您有新的客服消息，请查看")
	
	c.JSON(http.StatusOK, gin.H{"message": "推送提醒已发送"})
}

// checkPushStatus 检查推送配置状态
func checkPushStatus(c *gin.Context, db *gorm.DB) {
	csID := parseUint(c.Param("csId"))
	userID := parseUint(c.Param("userId"))
	
	if csID == 0 || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	
	// 验证用户是否存在
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 检查该用户的小程序是否分配给该客服
	var assignment models.Assignment
	if err := db.Where("mini_app_id = ? AND customer_service_id = ?", user.MiniAppID, csID).First(&assignment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该用户不属于您负责的小程序"})
		return
	}
	
	// 获取小程序信息
	var ma models.MiniApp
	if err := db.First(&ma, user.MiniAppID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"subscribed":    user.Subscribed,
			"miniAppExists": false,
			"templateId":    "",
			"message":       "小程序不存在",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"subscribed":    user.Subscribed,
		"miniAppExists": true,
		"appId":         ma.AppID,
		"templateId":     ma.TemplateID,
		"hasTemplateId": ma.TemplateID != "",
		"message":       "配置正常",
	})
}

// getCSQRCode 获取客服的小程序二维码
func getCSQRCode(c *gin.Context, db *gorm.DB) {
	csID := parseUint(c.Param("csId"))
	if csID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}
	
	// 获取客服信息
	var cs models.CustomerService
	if err := db.First(&cs, csID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "客服不存在"})
		return
	}
	
	// 获取二维码路径：优先使用客服单独设置的，如果没有则使用全局设置
	qrCodePath := cs.QRCodePath
	if qrCodePath == "" {
		// 获取全局二维码路径
		var config models.Config
		if err := db.Where("key = ?", "global_qrcode_path").First(&config).Error; err == nil {
			qrCodePath = config.Value
		}
	}
	
	if qrCodePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未设置二维码路径（请设置全局二维码路径或客服单独设置）"})
		return
	}
	
	// 获取分配给该客服的小程序（取第一个）
	var assignments []models.Assignment
	db.Where("customer_service_id = ?", csID).Find(&assignments)
	if len(assignments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该客服未分配小程序"})
		return
	}
	
	var miniApp models.MiniApp
	if err := db.First(&miniApp, assignments[0].MiniAppID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "小程序不存在"})
		return
	}
	
	// 获取 access_token
	tokenURL := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + miniApp.AppID + "&secret=" + miniApp.Secret
	resp, err := http.Get(tokenURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 access_token 失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)
	
	if tokenResp.ErrCode != 0 || tokenResp.AccessToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 access_token 失败: " + tokenResp.ErrMsg})
		return
	}
	
	// 调用微信API获取小程序码
	qrCodeURL := "https://api.weixin.qq.com/wxa/getwxacode?access_token=" + tokenResp.AccessToken
	qrCodeData := map[string]interface{}{
		"path": qrCodePath,
		"width": 280, // 二维码宽度，单位px，最小280px，最大1280px
	}
	jsonData, _ := json.Marshal(qrCodeData)
	
	qrResp, err := http.Post(qrCodeURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取二维码失败: " + err.Error()})
		return
	}
	defer qrResp.Body.Close()
	
	// 检查响应类型
	contentType := qrResp.Header.Get("Content-Type")
	if contentType == "application/json" {
		// 错误响应
		var errResp struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		json.NewDecoder(qrResp.Body).Decode(&errResp)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取二维码失败: " + errResp.ErrMsg})
		return
	}
	
	// 成功返回图片
	c.Data(http.StatusOK, "image/png", nil)
	io.Copy(c.Writer, qrResp.Body)
}

// userHeartbeat 用户心跳，更新最后活动时间
func userHeartbeat(c *gin.Context, db *gorm.DB) {
	var req struct {
		OpenID string `json:"openId"`
		AppID  string `json:"appId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	
	if req.OpenID == "" || req.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "openId 和 appId 不能为空"})
		return
	}
	
	var user models.User
	miniAppID := findMiniAppID(db, req.AppID)
	if miniAppID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该小程序"})
		return
	}
	
	if err := db.Where("open_id = ?", req.OpenID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 更新最后活动时间
	now := time.Now()
	user.LastActiveTime = &now
	db.Save(&user)
	
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
