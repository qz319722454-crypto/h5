#!/bin/bash

# 设置项目目录（根据您的服务器路径调整，如果不是 /www/wwwroot/h5-project，请修改）
PROJECT_DIR="/www/wwwroot/h5-project"

# 进入目录
cd $PROJECT_DIR

# 确保 backend 有 go.mod 和依赖
cd backend
export GOPROXY=https://goproxy.cn,direct
if [ ! -f go.mod ]; then go mod init h5-backend; fi
go get github.com/gin-gonic/gin gorm.io/gorm gorm.io/driver/mysql github.com/gorilla/websocket golang.org/x/crypto/bcrypt github.com/google/uuid
go mod tidy
go mod download
cd ..

# 停止并删除旧组成（如果存在）
docker-compose down || true

# 构建并启动所有服务（后端 + 数据库）
docker-compose up -d --build

# 等待 MySQL 启动（循环检查直到 mysqladmin ping 成功，最多 120秒）
echo "等待 MySQL 容器启动并就绪..."
for i in {1..120}; do
  if docker exec h5-mysql mysqladmin -uroot -prootpass ping -h localhost --silent &> /dev/null; then
    echo "MySQL 已就绪。"
    break
  fi
  sleep 1
done
if ! docker exec h5-mysql mysqladmin -uroot -prootpass ping -h localhost --silent &> /dev/null; then
  echo "错误: MySQL 未在 120秒内就绪。请检查日志: docker logs h5-mysql"
  exit 1
fi

# 初始化数据库
cat init_db.sql | docker exec -i h5-mysql mysql -uroot -prootpass customer_service_db --default-character-set=utf8mb4
echo "数据库初始化完成！"

# 确保管理员账号存在
echo "创建管理员账号..."
docker exec -i h5-mysql mysql -uroot -prootpass customer_service_db << 'ADMIN_SQL'
-- 添加 is_admin 列（如果不存在）
SET @dbname = DATABASE();
SET @tablename = 'customer_services';
SET @columnname = 'is_admin';
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tablename)
      AND (table_schema = @dbname)
      AND (column_name = @columnname)
  ) > 0,
  'SELECT 1',
  CONCAT('ALTER TABLE ', @tablename, ' ADD COLUMN ', @columnname, ' TINYINT(1) DEFAULT 0')
));
PREPARE alterIfNotExists FROM @preparedStatement;
EXECUTE alterIfNotExists;
DEALLOCATE PREPARE alterIfNotExists;
ADMIN_SQL

# 等待后端启动
echo "等待后端服务启动..."
sleep 10

# 使用后端API重置管理员密码（确保密码哈希正确）
echo "通过API重置管理员密码..."
for i in {1..30}; do
  if curl -s -X POST http://127.0.0.1:8080/admin/reset-admin \
    -H "Content-Type: application/json" \
    -d '{}' > /dev/null 2>&1; then
    echo "管理员密码已重置：用户名 admin，密码 admin"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "警告: 无法通过API重置密码，请手动运行: curl -X POST http://127.0.0.1:8080/admin/reset-admin"
  fi
  sleep 2
done

# 部署登录页面（静态文件，因为需要主域名直接访问）
echo "部署登录页面..."
mkdir -p /www/wwwroot/h5-project/public
if [ -f frontend/login.html ]; then
  cp frontend/login.html /www/wwwroot/h5-project/public/index.html
  chmod 755 /www/wwwroot/h5-project/public/index.html
  echo "登录页面部署成功。"
elif [ -f login.html ]; then
  cp login.html /www/wwwroot/h5-project/public/index.html
  chmod 755 /www/wwwroot/h5-project/public/index.html
  echo "登录页面部署成功。"
else
  echo "警告: login.html 不存在！请检查 frontend/login.html 或 login.html。"
fi

echo "前端容器（admin/cs）由 docker-compose 管理，访问 https://kefu.chacaitx.cn (如果证书配置好) 或 http://kefu.chacaitx.cn。"

# 配置 Nginx 反向代理
NGINX_CONF_PATH="/www/server/panel/vhost/nginx/kefu.chacaitx.cn.conf"
CERT_PATH="/www/server/panel/vhost/cert/kefu.chacaitx.cn/fullchain.pem"
if [ ! -f "$CERT_PATH" ]; then
  echo "警告: SSL 证书不存在 ($CERT_PATH)。使用临时 HTTP 配置。请在宝塔申请 Let's Encrypt 证书。"
  cat << 'EOF' > "$NGINX_CONF_PATH"
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

    # WebSocket 连接需要特殊处理，放在 /api/ 之前
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
        
        # WebSocket 超时设置（非常重要）
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_connect_timeout 60s;
        
        # 禁用缓冲，实时传输
        proxy_buffering off;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 普通 API 请求的超时设置
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
EOF
else
  cp nginx.conf "$NGINX_CONF_PATH"
fi

if [ -f "$NGINX_CONF_PATH" ]; then
  cp "$NGINX_CONF_PATH" "$NGINX_CONF_PATH.bak"
  echo "备份旧 Nginx 配置完成。"
fi

# 测试和重启...
if [ -f /www/server/nginx/sbin/nginx ]; then
  /www/server/nginx/sbin/nginx -t
  if [ $? -eq 0 ]; then
    echo "Nginx 配置测试成功。"
    /www/server/nginx/sbin/nginx -s reload
    echo "Nginx 已重启。"
  else
    echo "错误: Nginx 配置测试失败。请检查 nginx.conf。"
    exit 1
  fi

  # 测试访问 /admin/index.html with verbose output
  echo "测试访问 https://kefu.chacaitx.cn/admin/index.html (verbose)..."
  curl -v https://kefu.chacaitx.cn/admin/index.html > /dev/null 2>&1 | grep -E "HTTP/|X-Debug-Location|Content-Type"

  # If X-Debug-Location is "proxy-api", it's being proxied incorrectly
  # 测试后端代理 (e.g., /api/admin/login)
  echo "测试后端代理 https://kefu.chacaitx.cn/api/admin/login ..."
  API_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" https://kefu.chacaitx.cn/api/admin/login)
  if [ "$API_RESPONSE" = "405" ] || [ "$API_RESPONSE" = "200" ]; then  # 405 for GET on POST endpoint is expected
    echo "代理测试成功: 返回 $API_RESPONSE。"
  else
    echo "代理测试失败: 返回 $API_RESPONSE。"
  fi

  # 检查 Nginx 错误日志
  echo "检查 Nginx 错误日志（最后10行）："
  tail -n 10 /www/server/panel/logs/nginx_error.log || echo "日志文件未找到。"
  
  echo "按 Enter 继续或检查浏览器。" ; read
else
  echo "警告: 未找到 Nginx，无法自动重启。请手动重启。"
fi

echo "Nginx 代理配置完成！"

echo "一键部署完成！"
echo "- 后端运行在端口 8080。"
echo "- 数据库运行在端口 3307（用户: root, 密码: rootpass）。"
echo "检查日志：docker-compose logs"
echo "如果需要停止：docker-compose down"
