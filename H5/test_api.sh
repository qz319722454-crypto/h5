#!/bin/bash

echo "=== 测试 API 连接 ==="
echo ""

# 测试后端是否运行
echo "1. 检查后端服务状态..."
if docker ps | grep -q h5-backend; then
    echo "✓ 后端容器正在运行"
else
    echo "✗ 后端容器未运行，请先启动: docker-compose up -d backend"
    exit 1
fi

# 测试后端直接访问
echo ""
echo "2. 测试后端直接访问 (http://127.0.0.1:8080/chat/login)..."
response=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:8080/chat/login \
  -H "Content-Type: application/json" \
  -d '{"code":"test","appId":"test"}')
echo "   响应状态码: $response"
if [ "$response" = "404" ]; then
    echo "   ✗ 后端路由不存在，检查后端日志"
    docker logs h5-backend --tail 20
elif [ "$response" = "400" ] || [ "$response" = "200" ]; then
    echo "   ✓ 后端路由存在（400/200 表示路由正常，只是参数错误）"
else
    echo "   ? 未知响应: $response"
fi

# 测试通过 Nginx 访问
echo ""
echo "3. 测试通过 Nginx 访问 (https://kefu.chacaitx.cn/api/chat/login)..."
response=$(curl -s -o /dev/null -w "%{http_code}" -X POST https://kefu.chacaitx.cn/api/chat/login \
  -H "Content-Type: application/json" \
  -d '{"code":"test","appId":"test"}' \
  -k)
echo "   响应状态码: $response"
if [ "$response" = "404" ]; then
    echo "   ✗ Nginx 代理失败，检查 Nginx 配置"
    echo "   检查 Nginx 错误日志:"
    tail -5 /www/server/panel/logs/nginx_error.log
elif [ "$response" = "400" ] || [ "$response" = "200" ]; then
    echo "   ✓ Nginx 代理正常（400/200 表示路由正常，只是参数错误）"
else
    echo "   ? 未知响应: $response"
fi

# 检查 Nginx 配置
echo ""
echo "4. 检查 Nginx 配置..."
if /www/server/nginx/sbin/nginx -t 2>&1 | grep -q "successful"; then
    echo "   ✓ Nginx 配置正确"
else
    echo "   ✗ Nginx 配置有错误:"
    /www/server/nginx/sbin/nginx -t
fi

# 检查后端路由
echo ""
echo "5. 检查后端路由..."
echo "   查看后端日志中的路由列表:"
docker logs h5-backend 2>&1 | grep -i "GIN-debug" | grep -i "chat" | tail -10

echo ""
echo "=== 测试完成 ==="

