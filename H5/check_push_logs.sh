#!/bin/bash

# 查看推送日志的脚本

echo "=========================================="
echo "查看后端推送日志（最近50条）"
echo "=========================================="
echo ""

# 检查 Docker 容器是否存在
if ! docker ps | grep -q h5-backend; then
    echo "❌ 错误: h5-backend 容器未运行"
    echo ""
    echo "请先启动容器:"
    echo "  docker-compose up -d"
    exit 1
fi

echo "正在查看推送日志..."
echo ""

# 查看包含"推送"关键词的日志
docker logs h5-backend --tail 200 | grep -E "\[推送\]" | tail -50

echo ""
echo "=========================================="
echo "提示:"
echo "- 如果看到 '✅ 推送成功'，说明推送已成功发送"
echo "- 如果看到 '❌ 推送失败'，请查看错误码和错误信息"
echo "- 常见错误码:"
echo "  43101: 用户拒绝接受消息"
echo "  47003: 参数错误"
echo "  40037: 模板ID不正确"
echo "  45009: 接口调用超过限制（频率限制）"
echo "=========================================="

