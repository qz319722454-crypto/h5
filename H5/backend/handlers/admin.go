package handlers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"h5-backend/models"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"time"
	"strconv"
	"strings"
)

// SetupAdminRoutes sets up routes for admin operations
func SetupAdminRoutes(r *gin.Engine, db *gorm.DB) {
	admin := r.Group("/admin")
	{
		admin.POST("/miniapp", func(c *gin.Context) { addMiniApp(c, db) })
		admin.GET("/miniapps", func(c *gin.Context) { getMiniApps(c, db) })
		admin.POST("/cs", func(c *gin.Context) { addCustomerService(c, db) })
		admin.GET("/cs", func(c *gin.Context) { getCustomerServices(c, db) })
		admin.POST("/assign", func(c *gin.Context) { assignMiniAppToCS(c, db) })
		admin.GET("/assignments", func(c *gin.Context) { getAssignments(c, db) })
		admin.DELETE("/miniapp/:id", func(c *gin.Context) { deleteMiniApp(c, db) })
		admin.DELETE("/cs/:id", func(c *gin.Context) { deleteCustomerService(c, db) })
		admin.DELETE("/assign/:id", func(c *gin.Context) { deleteAssignment(c, db) })
		admin.POST("/login", func(c *gin.Context) { csLogin(c, db) })
		admin.POST("/reset-admin", func(c *gin.Context) { resetAdminPassword(c, db) })
		admin.GET("/cs/:id/miniapps", func(c *gin.Context) { getCSMiniApps(c, db) })
		admin.GET("/cs/:id/users", func(c *gin.Context) { getCSUsers(c, db) })
		admin.PUT("/cs/:id/qrcode", func(c *gin.Context) { updateCSQRCodePath(c, db) })
		admin.PUT("/cs/:id/welcome", func(c *gin.Context) { updateCSWelcomeMessage(c, db) })
		admin.GET("/cs/:id/welcome", func(c *gin.Context) { getCSWelcomeMessage(c, db) })
		admin.DELETE("/cs/:id/user/:userId", func(c *gin.Context) { deleteUser(c, db) })
		admin.PUT("/config/global-qrcode", func(c *gin.Context) { updateGlobalQRCodePath(c, db) })
		admin.GET("/config/global-qrcode", func(c *gin.Context) { getGlobalQRCodePath(c, db) })
	}
}

func addMiniApp(c *gin.Context, db *gorm.DB) {
	var miniApp models.MiniApp
	if err := c.ShouldBindJSON(&miniApp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if miniApp.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "小程序名称不能为空"})
		return
	}
	if miniApp.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "App ID 不能为空"})
		return
	}
	
	// 检查 AppID 是否已存在
	var existingApp models.MiniApp
	if err := db.Where("app_id = ?", miniApp.AppID).First(&existingApp).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该 AppID 已存在，无法重复添加"})
		return
	}
	
	if err := db.Create(&miniApp).Error; err != nil {
		// 检查是否是重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE constraint") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该 AppID 已存在，无法重复添加"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, miniApp)
}

func addCustomerService(c *gin.Context, db *gorm.DB) {
	var req struct {
		Name     string `json:"Name"`
		Password string `json:"Password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if req.Name == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	
	cs := models.CustomerService{
		Name:     req.Name,
		Password: req.Password,
	}
	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(cs.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	cs.Password = string(hashed)
	if err := db.Create(&cs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, cs)
}

func assignMiniAppToCS(c *gin.Context, db *gorm.DB) {
	var assignment models.Assignment
	if err := c.ShouldBindJSON(&assignment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if assignment.MiniAppID == 0 || assignment.CustomerServiceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "小程序ID和客服ID不能为空"})
		return
	}
	
	// 检查该小程序是否已经分配给其他客服
	var existingAssignment1 models.Assignment
	if err := db.Where("mini_app_id = ?", assignment.MiniAppID).First(&existingAssignment1).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该小程序已分配给其他客服，每个小程序只能分配一个客服"})
		return
	}
	
	// 检查该客服是否已经分配了其他小程序
	var existingAssignment2 models.Assignment
	if err := db.Where("customer_service_id = ?", assignment.CustomerServiceID).First(&existingAssignment2).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该客服已分配了其他小程序，每个客服只能分配一个小程序"})
		return
	}
	
	if err := db.Create(&assignment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, assignment)
}

func csLogin(c *gin.Context, db *gorm.DB) {
	var req struct {
		Name     string `json:"Name"`
		Password string `json:"Password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.Name == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	var cs models.CustomerService
	if err := db.Where("name = ?", req.Name).First(&cs).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	// Compare hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(cs.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"csId": cs.ID, "isAdmin": cs.IsAdmin})
}

// resetAdminPassword 临时端点：重置管理员密码（仅用于初始化）
func resetAdminPassword(c *gin.Context, db *gorm.DB) {
	var cs models.CustomerService
	db.Where("name = ?", "admin").First(&cs)
	
	// 生成新的密码哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	
	if cs.ID == 0 {
		// 创建管理员账号
		cs = models.CustomerService{
			Name:     "admin",
			Password: string(hashed),
			IsAdmin:  true,
		}
		if err := db.Create(&cs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建管理员账号失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "管理员账号已创建，密码: admin"})
	} else {
		// 更新密码
		cs.Password = string(hashed)
		cs.IsAdmin = true
		if err := db.Save(&cs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "管理员密码已重置为: admin"})
	}
}

// getMiniApps 获取小程序列表
func getMiniApps(c *gin.Context, db *gorm.DB) {
	var miniApps []models.MiniApp
	db.Find(&miniApps)
	c.JSON(http.StatusOK, miniApps)
}

// getCustomerServices 获取客服列表
func getCustomerServices(c *gin.Context, db *gorm.DB) {
	var csList []models.CustomerService
	db.Select("id, name, is_admin, qr_code_path, welcome_message, created_at, updated_at").Find(&csList)
	c.JSON(http.StatusOK, csList)
}

// updateCSQRCodePath 更新客服的二维码路径
func updateCSQRCodePath(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	var req struct {
		QRCodePath string `json:"QRCodePath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	
	var cs models.CustomerService
	if err := db.First(&cs, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "客服不存在"})
		return
	}
	
	cs.QRCodePath = req.QRCodePath
	if err := db.Save(&cs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "cs": cs})
}

// updateCSWelcomeMessage 更新客服的欢迎语
func updateCSWelcomeMessage(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	var req struct {
		WelcomeMessage string `json:"WelcomeMessage"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	
	var cs models.CustomerService
	if err := db.First(&cs, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "客服不存在"})
		return
	}
	
	cs.WelcomeMessage = req.WelcomeMessage
	if err := db.Save(&cs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "cs": cs})
}

// getCSWelcomeMessage 获取客服的欢迎语
func getCSWelcomeMessage(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	var cs models.CustomerService
	if err := db.Select("id, welcome_message").First(&cs, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "客服不存在"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"WelcomeMessage": cs.WelcomeMessage})
}

// getAssignments 获取分配列表
func getAssignments(c *gin.Context, db *gorm.DB) {
	var assignments []models.Assignment
	db.Find(&assignments)
	c.JSON(http.StatusOK, assignments)
}

// deleteMiniApp 删除小程序（级联删除相关数据）
func deleteMiniApp(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	miniAppIDUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的小程序ID"})
		return
	}
	miniAppID := uint(miniAppIDUint)
	if miniAppID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的小程序ID"})
		return
	}
	
	// 检查小程序是否存在
	var miniApp models.MiniApp
	if err := db.First(&miniApp, miniAppID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "小程序不存在"})
		return
	}
	
	// 1. 获取该小程序下的所有用户
	var users []models.User
	db.Where("mini_app_id = ?", miniAppID).Find(&users)
	
	// 2. 删除这些用户的所有消息（硬删除）
	if len(users) > 0 {
		var userIDs []uint
		for _, user := range users {
			userIDs = append(userIDs, user.ID)
		}
		// 硬删除消息（不是软删除）
		db.Unscoped().Where("user_id IN ?", userIDs).Delete(&models.Message{})
	}
	
	// 3. 删除该小程序下的所有用户（硬删除）
	db.Unscoped().Where("mini_app_id = ?", miniAppID).Delete(&models.User{})
	
	// 4. 删除该小程序的所有分配关系（硬删除）
	db.Unscoped().Where("mini_app_id = ?", miniAppID).Delete(&models.Assignment{})
	
	// 5. 最后删除小程序本身（硬删除）
	if err := db.Unscoped().Delete(&miniApp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功，已删除所有相关数据"})
}

// deleteCustomerService 删除客服（级联删除相关数据）
func deleteCustomerService(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	csIDUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}
	csID := uint(csIDUint)
	if csID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}
	
	// 检查客服是否存在
	var cs models.CustomerService
	if err := db.First(&cs, csID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "客服不存在"})
		return
	}
	
	// 1. 删除该客服的所有消息（硬删除）
	db.Unscoped().Where("customer_service_id = ?", csID).Delete(&models.Message{})
	
	// 2. 删除该客服的所有分配关系（硬删除）
	db.Unscoped().Where("customer_service_id = ?", csID).Delete(&models.Assignment{})
	
	// 3. 最后删除客服本身（硬删除）
	if err := db.Unscoped().Delete(&cs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功，已删除所有相关数据"})
}

// deleteAssignment 删除分配
func deleteAssignment(c *gin.Context, db *gorm.DB) {
	id := c.Param("id")
	if err := db.Delete(&models.Assignment{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// getCSMiniApps 获取分配给客服的小程序列表
func getCSMiniApps(c *gin.Context, db *gorm.DB) {
	csIDStr := c.Param("id")
	csIDUint, err := strconv.ParseUint(csIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}
	csID := uint(csIDUint)
	
	var assignments []models.Assignment
	db.Where("customer_service_id = ?", csID).Find(&assignments)
	
	var miniAppIDs []uint
	for _, assign := range assignments {
		miniAppIDs = append(miniAppIDs, assign.MiniAppID)
	}
	
	var miniApps []models.MiniApp
	if len(miniAppIDs) > 0 {
		db.Where("id IN ?", miniAppIDs).Find(&miniApps)
	}
	
	c.JSON(http.StatusOK, miniApps)
}

// getCSUsers 获取分配给客服的用户列表（带小程序信息）
func getCSUsers(c *gin.Context, db *gorm.DB) {
	csIDStr := c.Param("id")
	csIDUint, err := strconv.ParseUint(csIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客服ID"})
		return
	}
	csID := uint(csIDUint)
	
	// 获取分配给该客服的小程序ID列表
	var assignments []models.Assignment
	db.Where("customer_service_id = ?", csID).Find(&assignments)
	
	var miniAppIDs []uint
	for _, assign := range assignments {
		miniAppIDs = append(miniAppIDs, assign.MiniAppID)
	}
	
	if len(miniAppIDs) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	
	// 获取这些小程序下的所有用户
	var users []models.User
	db.Where("mini_app_id IN ?", miniAppIDs).Find(&users)
	
	// 获取小程序信息
	var miniApps []models.MiniApp
	db.Where("id IN ?", miniAppIDs).Find(&miniApps)
	miniAppMap := make(map[uint]models.MiniApp)
	for _, ma := range miniApps {
		miniAppMap[ma.ID] = ma
	}
	
	// 获取每个用户的最后一条消息和未读消息数
	type UserWithInfo struct {
		models.User
		MiniAppName string `json:"MiniAppName"`
		LastMessage string `json:"LastMessage"`
		LastMessageTime string `json:"LastMessageTime"`
		UnreadCount int `json:"UnreadCount"`
		Subscribed bool `json:"Subscribed"` // 是否已授权订阅消息
		IsOnline bool `json:"IsOnline"` // 是否在线（5分钟内有活动）
	}
	
	var result []UserWithInfo
	for _, user := range users {
		ma, ok := miniAppMap[user.MiniAppID]
		miniAppName := "未分配"
		if ok {
			// 优先显示小程序名称，如果没有名称则显示 AppID
			if ma.Name != "" {
				miniAppName = ma.Name
			} else {
				miniAppName = ma.AppID
			}
		}
		
		// 获取最后一条消息
		var lastMsg models.Message
		db.Where("user_id = ? AND customer_service_id = ?", user.ID, csID).
			Order("created_at DESC").First(&lastMsg)
		
		lastMessage := ""
		lastMessageTime := ""
		if lastMsg.ID > 0 {
			if lastMsg.IsImage {
				lastMessage = "[图片]"
			} else {
				lastMessage = lastMsg.Content
				if len(lastMessage) > 30 {
					lastMessage = lastMessage[:30] + "..."
				}
			}
			lastMessageTime = lastMsg.CreatedAt.Format("2006-01-02 15:04:05")
		}
		
		// 统计未读消息数（用户发送的，客服未读的）
		var unreadCount int64
		db.Model(&models.Message{}).
			Where("user_id = ? AND customer_service_id = ? AND from_user = ? AND is_read = ?", user.ID, csID, true, false).
			Count(&unreadCount)
		
		// 判断用户是否在线（1分钟内有活动，实时检测）
		isOnline := false
		if user.LastActiveTime != nil {
			timeSinceActive := time.Since(*user.LastActiveTime)
			isOnline = timeSinceActive < 1*time.Minute
		}
		
		result = append(result, UserWithInfo{
			User: user,
			MiniAppName: miniAppName,
			LastMessage: lastMessage,
			LastMessageTime: lastMessageTime,
			UnreadCount: int(unreadCount),
			Subscribed: user.Subscribed,
			IsOnline: isOnline,
		})
	}
	
	c.JSON(http.StatusOK, result)
}

// deleteUser 删除用户（客服端）
func deleteUser(c *gin.Context, db *gorm.DB) {
	csIDStr := c.Param("id")
	userIDStr := c.Param("userId")
	csID, _ := strconv.ParseUint(csIDStr, 10, 32)
	userID, _ := strconv.ParseUint(userIDStr, 10, 32)
	
	if csID == 0 || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	
	// 验证用户是否属于分配给该客服的小程序
	var user models.User
	if err := db.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	
	// 检查该用户的小程序是否分配给该客服
	var assignment models.Assignment
	if err := db.Where("mini_app_id = ? AND customer_service_id = ?", user.MiniAppID, uint(csID)).First(&assignment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该用户不属于您负责的小程序"})
		return
	}
	
	// 删除用户的所有消息（硬删除）
	db.Unscoped().Where("user_id = ?", uint(userID)).Delete(&models.Message{})
	
	// 删除用户（硬删除）
	if err := db.Unscoped().Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "用户已删除，已删除所有相关消息"})
}

// updateGlobalQRCodePath 更新全局二维码路径
func updateGlobalQRCodePath(c *gin.Context, db *gorm.DB) {
	var req struct {
		QRCodePath string `json:"QRCodePath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	
	// 查找或创建配置
	var config models.Config
	if err := db.Where("key = ?", "global_qrcode_path").First(&config).Error; err != nil {
		// 不存在，创建新配置
		config = models.Config{
			Key:   "global_qrcode_path",
			Value: req.QRCodePath,
		}
		if err := db.Create(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "设置失败: " + err.Error()})
			return
		}
	} else {
		// 存在，更新配置
		config.Value = req.QRCodePath
		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "设置失败: " + err.Error()})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "全局二维码路径设置成功", "QRCodePath": config.Value})
}

// getGlobalQRCodePath 获取全局二维码路径
func getGlobalQRCodePath(c *gin.Context, db *gorm.DB) {
	var config models.Config
	if err := db.Where("key = ?", "global_qrcode_path").First(&config).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"QRCodePath": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"QRCodePath": config.Value})
}

// parseUint 解析字符串为uint（在chat.go中已定义，这里不再重复）
// func parseUint(s string) uint {
// 	u, _ := strconv.ParseUint(s, 10, 32)
// 	return uint(u)
// }

// TODO: Add more handlers for chat matching, WebSocket, etc.
