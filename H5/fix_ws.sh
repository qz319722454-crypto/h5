#!/bin/bash

NGINX_CONF_PATH="/www/server/panel/vhost/nginx/kefu.chacaitx.cn.conf"

echo "修复 WebSocket 配置..."

# 备份
cp "$NGINX_CONF_PATH" "$NGINX_CONF_PATH.bak.$(date +%Y%m%d_%H%M%S)"

# 检查当前配置
echo "当前 WebSocket location 配置："
grep -A 3 "location.*chat/ws" "$NGINX_CONF_PATH" || echo "未找到 WebSocket location"

# 删除旧的 WebSocket location（如果有）
sed -i '/location.*chat\/ws/,/^[[:space:]]*}$/d' "$NGINX_CONF_PATH"

# 在 /api/ location 之前插入正确的 WebSocket 配置
# 找到 /api/ location 的行号
API_LINE=$(grep -n "^[[:space:]]*location /api/" "$NGINX_CONF_PATH" | head -1 | cut -d: -f1)

if [ -n "$API_LINE" ]; then
    echo "在行 $API_LINE 之前插入 WebSocket 配置..."
    # 在 /api/ location 之前插入
    sed -i "${API_LINE}i\\
    # WebSocket 连接需要特殊处理\\
    location ~ ^/api/chat/ws/(.+)\$ {\\
        proxy_pass http://127.0.0.1:8080/chat/ws/\$1;\\
        proxy_http_version 1.1;\\
        proxy_set_header Upgrade \$http_upgrade;\\
        proxy_set_header Connection \"upgrade\";\\
        proxy_set_header Host \$host;\\
        proxy_set_header X-Real-IP \$remote_addr;\\
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;\\
        proxy_set_header X-Forwarded-Proto \$scheme;\\
        proxy_read_timeout 3600s;\\
        proxy_send_timeout 3600s;\\
        proxy_connect_timeout 60s;\\
        proxy_buffering off;\\
    }\\
" "$NGINX_CONF_PATH"
else
    echo "未找到 /api/ location，使用完整配置覆盖"
    # 如果找不到，直接使用 clean_nginx.sh
    ./clean_nginx.sh
    exit 0
fi

# 测试配置
echo "测试 Nginx 配置..."
if /www/server/nginx/sbin/nginx -t; then
    echo "✓ 配置测试通过"
    /www/server/nginx/sbin/nginx -s reload
    echo "✓ Nginx 已重载"
else
    echo "✗ 配置测试失败"
    /www/server/nginx/sbin/nginx -t
    exit 1
fi

echo "完成！"

