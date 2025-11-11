// pages/index/index.js
Page({
  data: {
    message: '',
    openId: '', // Set after login
    messages: [], // Array to hold chat history
    templateId: '', // 订阅消息模板ID
    hasRequestedAuth: false, // 是否已请求过授权
    appId: '' // 小程序AppID
  },
  onLoad: function() {
    const app = getApp();
    const appId = app.globalData.appId || 'your-app-id'; // 从全局获取或使用占位符
    this.setData({ appId: appId });
    
    wx.login({
      success: res => {
        wx.request({
          url: 'https://kefu.chacaitx.cn/api/chat/login',
          method: 'POST',
          data: { code: res.code, appId: appId },
          success: res => {
            if (res.data.openId) {
              this.setData({ 
                openId: res.data.openId,
                templateId: res.data.templateId || '' // 从后端获取模板ID
              });
              // 发送心跳
              this.sendHeartbeat();
            }
          }
        })
      }
    })
    this.fetchHistory();
    // Poll for new messages every 5 seconds
    this.messageTimer = setInterval(() => {
      this.fetchHistory();
    }, 5000);
    // 每30秒发送一次心跳
    this.heartbeatTimer = setInterval(() => {
      if (this.data.openId && this.data.appId) {
        this.sendHeartbeat();
      }
    }, 30000);
  },
  onUnload: function() {
    // 清除定时器
    if (this.messageTimer) {
      clearInterval(this.messageTimer);
    }
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
    }
  },
  sendHeartbeat: function() {
    if (!this.data.openId || !this.data.appId) return;
    
    wx.request({
      url: 'https://kefu.chacaitx.cn/api/chat/heartbeat',
      method: 'POST',
      data: {
        openId: this.data.openId,
        appId: this.data.appId
      },
      success: res => {
        // 心跳成功，静默处理
      },
      fail: err => {
        console.error('心跳失败:', err);
      }
    });
  },
  fetchHistory: function() {
    if (!this.data.openId || !this.data.appId) {
      return; // 如果还没有openId，不请求历史记录
    }
    wx.request({
      url: 'https://kefu.chacaitx.cn/api/chat/history',
      data: { openId: this.data.openId, appId: this.data.appId },
      success: res => {
        if (res.statusCode === 200 && res.data) {
          this.setData({ messages: res.data });
        }
      }
    });
  },
  bindMessage: function(e) {
    this.setData({ message: e.detail.value });
  },
  // 请求订阅消息授权（自动调用）
  requestSubscriptionAuth: function() {
    if (this.data.hasRequestedAuth || !this.data.templateId) {
      return; // 已经请求过或没有模板ID，不再请求
    }
    
    this.setData({ hasRequestedAuth: true });
    
    wx.requestSubscribeMessage({
      tmplIds: [this.data.templateId],
      success: res => {
        const templateId = this.data.templateId;
        if (res[templateId] === 'accept') {
          wx.request({
            url: 'https://kefu.chacaitx.cn/api/chat/subscribe',
            method: 'POST',
            data: { openId: this.data.openId },
            success: res => {
              console.log('订阅状态已更新');
              wx.showToast({
                title: '授权成功，将及时收到客服回复',
                icon: 'success',
                duration: 2000
              });
            },
            fail: err => {
              console.error('更新订阅状态失败:', err);
            }
          });
        } else if (res[templateId] === 'reject') {
          wx.showToast({
            title: '已拒绝授权，将无法收到推送通知',
            icon: 'none',
            duration: 2000
          });
        } else if (res[templateId] === 'ban') {
          wx.showToast({
            title: '已被禁止授权',
            icon: 'none',
            duration: 2000
          });
        }
      },
      fail: err => {
        console.error('订阅消息授权失败:', err);
        wx.showToast({
          title: '授权失败，请重试',
          icon: 'none',
          duration: 2000
        });
        this.setData({ hasRequestedAuth: false }); // 失败后允许重试
      }
    });
  },
  sendMessage: function() {
    if (!this.data.message.trim()) {
      wx.showToast({
        title: '请输入消息内容',
        icon: 'none'
      });
      return;
    }
    
    // 首次发送消息时自动请求授权
    if (!this.data.hasRequestedAuth && this.data.templateId) {
      this.requestSubscriptionAuth();
    }
    
    wx.request({
      url: 'https://kefu.chacaitx.cn/api/chat/send',
      method: 'POST',
      data: {
        appId: this.data.appId,
        openId: this.data.openId,
        content: this.data.message
      },
      success: res => {
        if (res.statusCode === 200) {
          this.setData({ message: '' }); // 清空输入框
          this.fetchHistory(); // 刷新消息列表
          // 发送消息时也会更新活动时间，但再发送一次心跳确保及时更新
          this.sendHeartbeat();
        } else {
          wx.showToast({
            title: '发送失败',
            icon: 'none'
          });
        }
      },
      fail: err => {
        wx.showToast({
          title: '网络错误',
          icon: 'none'
        });
      }
    });
  },
  sendImage: function() {
    // 首次发送消息时自动请求授权
    if (!this.data.hasRequestedAuth && this.data.templateId) {
      this.requestSubscriptionAuth();
    }
    
    const that = this;
    wx.chooseImage({
      count: 1,
      sizeType: ['original', 'compressed'],
      sourceType: ['album', 'camera'],
      success: res => {
        wx.uploadFile({
          url: 'https://kefu.chacaitx.cn/api/chat/upload',
          filePath: res.tempFilePaths[0],
          name: 'image',
          success: uploadRes => {
            const data = JSON.parse(uploadRes.data);
            if (data.url) {
              wx.request({
                url: 'https://kefu.chacaitx.cn/api/chat/send',
                method: 'POST',
                data: {
                  appId: that.data.appId,
                  openId: that.data.openId,
                  imageUrl: data.url
                },
                success: res => {
                  if (res.statusCode === 200) {
                    that.fetchHistory(); // 刷新消息列表
                    // 发送图片时也会更新活动时间，但再发送一次心跳确保及时更新
                    that.sendHeartbeat();
                  } else {
                    wx.showToast({
                      title: '发送失败',
                      icon: 'none'
                    });
                  }
                },
                fail: err => {
                  wx.showToast({
                    title: '网络错误',
                    icon: 'none'
                  });
                }
              });
            } else {
              wx.showToast({
                title: '上传失败',
                icon: 'none'
              });
            }
          },
          fail: err => {
            wx.showToast({
              title: '上传失败',
              icon: 'none'
            });
          }
        });
      }
    });
  }
});
