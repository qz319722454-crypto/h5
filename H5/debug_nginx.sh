#!/bin/bash

echo "=== 调试 Nginx WebSocket 配置 ==="
echo ""

# 检查配置文件
NGINX_CONF="/www/server/panel/vhost/nginx/kefu.chacaitx.cn.conf"

echo "1. 检查 WebSocket location 配置："
grep -A 15 "location.*chat/ws" "$NGINX_CONF"
echo ""

echo "2. 检查 location 顺序："
grep -n "location" "$NGINX_CONF"
echo ""

echo "3. 测试 WebSocket 路径匹配（模拟）："
echo "   请求路径: /api/chat/ws/8"
echo "   应该匹配: location ~ ^/api/chat/ws/(.+)$"
echo "   应该代理到: http://127.0.0.1:8080/chat/ws/8"
echo ""

echo "4. 检查 Nginx 错误日志（最近的 WebSocket 相关）："
tail -20 /www/server/panel/logs/nginx_error.log | grep -i "ws\|websocket" || echo "   无 WebSocket 相关错误"
echo ""

echo "5. 检查 Nginx 访问日志（最近的请求）："
tail -10 /www/server/panel/logs/nginx_access.log | grep "chat/ws" || echo "   无 WebSocket 访问记录"
echo ""

echo "6. 测试配置语法："
/www/server/nginx/sbin/nginx -t
echo ""

echo "7. 建议：如果配置正确但仍不工作，尝试："
echo "   - 完全重启 Nginx: /www/server/nginx/sbin/nginx -s stop && /www/server/nginx/sbin/nginx"
echo "   - 或者检查是否有其他配置文件覆盖了这个配置"

