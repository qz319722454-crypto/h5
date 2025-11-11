#!/bin/bash

# 为 mini_apps 表添加 name 字段的脚本

echo "=========================================="
echo "为 mini_apps 表添加 name 字段"
echo "=========================================="
echo ""

# 获取 MySQL 容器名称
MYSQL_CONTAINER="h5-mysql"
DB_NAME="customer_service_db"
DB_USER="root"
DB_PASSWORD="rootpass"

# 检查 Docker 容器是否存在
if ! docker ps | grep -q h5-mysql; then
    echo "❌ 错误: h5-mysql 容器未运行"
    echo ""
    echo "请先启动容器:"
    echo "  docker-compose up -d"
    exit 1
fi

echo "正在添加 name 字段..."
echo ""

# 执行 SQL 脚本
docker exec -i -e MYSQL_PWD=$DB_PASSWORD $MYSQL_CONTAINER mysql -u$DB_USER $DB_NAME < add_miniapp_name_column.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ name 字段添加完成！"
    echo ""
    echo "注意："
    echo "- 如果小程序没有名称，会显示 AppID"
    echo "- 建议在管理后台为现有小程序添加名称"
else
    echo ""
    echo "❌ 添加失败，请检查错误信息"
    exit 1
fi

