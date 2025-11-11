#!/bin/bash

NGINX_CONF_PATH="/www/server/panel/vhost/nginx/kefu.chacaitx.cn.conf"
CERT_PATH="/www/server/panel/vhost/cert/kefu.chacaitx.cn/fullchain.pem"

echo "正在清理并重新生成 Nginx 配置..."

# 备份
if [ -f "$NGINX_CONF_PATH" ]; then
    cp "$NGINX_CONF_PATH" "$NGINX_CONF_PATH.bak.$(date +%Y%m%d_%H%M%S)"
    echo "已备份原配置文件"
fi

# 检查证书是否存在
if [ -f "$CERT_PATH" ]; then
    echo "使用 HTTPS 配置..."
    cat > "$NGINX_CONF_PATH" << 'NGINX_EOF'
server {
    listen 80;
    listen 443 ssl;
    http2 on;
    server_name kefu.chacaitx.cn;

    ssl_certificate /www/server/panel/vhost/cert/kefu.chacaitx.cn/fullchain.pem;
    ssl_certificate_key /www/server/panel/vhost/cert/kefu.chacaitx.cn/privkey.pem;
    ssl_session_timeout 5m;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:HIGH:!aNULL:!MD5:!RC4:!DHE;
    ssl_prefer_server_ciphers on;

    if ($scheme != "https") {
        return 301 https://$host$request_uri;
    }

    location / {
        root /www/wwwroot/h5-project/public;
        index index.html login.html index.html index.htm;
        try_files $uri $uri/ /index.html =404;
        autoindex off;
        access_log /www/server/panel/logs/nginx_access.log;
        error_log /www/server/panel/logs/nginx_error.log warn;
        add_header Content-Type text/html;
    }

    location /admin/ {
        proxy_pass http://127.0.0.1:8081/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /cs/ {
        proxy_pass http://127.0.0.1:8082/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

        location /uploads/ {
            proxy_pass http://127.0.0.1:8080/uploads/;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            # 允许跨域访问图片
            add_header Access-Control-Allow-Origin *;
        }

    location /api/chat/ws/ {
        rewrite ^/api/chat/ws/(.+)$ /chat/ws/$1 break;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_connect_timeout 60s;
        proxy_buffering off;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
NGINX_EOF
else
    echo "使用 HTTP 配置（证书不存在）..."
    cat > "$NGINX_CONF_PATH" << 'NGINX_EOF'
server {
    listen 80;
    server_name kefu.chacaitx.cn;

    location / {
        root /www/wwwroot/h5-project/public;
        index index.html login.html index.html index.htm;
        try_files $uri $uri/ /index.html =404;
        autoindex off;
        add_header Content-Type text/html;
    }

    location /admin/ {
        proxy_pass http://127.0.0.1:8081/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /cs/ {
        proxy_pass http://127.0.0.1:8082/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

        location /uploads/ {
            proxy_pass http://127.0.0.1:8080/uploads/;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            # 允许跨域访问图片
            add_header Access-Control-Allow-Origin *;
        }

    location /api/chat/ws/ {
        rewrite ^/api/chat/ws/(.+)$ /chat/ws/$1 break;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_connect_timeout 60s;
        proxy_buffering off;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
NGINX_EOF
fi

# 测试配置
echo "测试 Nginx 配置..."
if /www/server/nginx/sbin/nginx -t; then
    echo "✓ Nginx 配置测试通过"
    echo "正在重新加载 Nginx..."
    /www/server/nginx/sbin/nginx -s reload
    echo "✓ Nginx 已重新加载"
    echo ""
    echo "配置已修复！现在可以测试 API 了。"
else
    echo "✗ Nginx 配置测试失败："
    /www/server/nginx/sbin/nginx -t
    exit 1
fi

