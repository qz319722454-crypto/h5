#!/bin/bash

echo "=== 完全重启 Nginx 以应用 WebSocket 配置 ==="
echo ""

# 测试配置
echo "1. 测试 Nginx 配置..."
if ! /www/server/nginx/sbin/nginx -t; then
    echo "✗ 配置测试失败，请先修复错误"
    exit 1
fi

echo "✓ 配置测试通过"
echo ""

# 停止 Nginx
echo "2. 停止 Nginx..."
/www/server/nginx/sbin/nginx -s stop
sleep 2

# 启动 Nginx
echo "3. 启动 Nginx..."
/www/server/nginx/sbin/nginx

# 检查是否启动成功
sleep 1
if pgrep -x nginx > /dev/null; then
    echo "✓ Nginx 已成功启动"
else
    echo "✗ Nginx 启动失败"
    exit 1
fi

echo ""
echo "4. 验证 WebSocket 配置..."
grep -A 3 "location ~.*chat/ws" /www/server/panel/vhost/nginx/kefu.chacaitx.cn.conf

echo ""
echo "完成！现在可以测试 WebSocket 连接了。"

