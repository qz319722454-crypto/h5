#!/bin/bash

# 清除所有小程序及其相关数据的脚本
# 使用方法：./clear_miniapps.sh

echo "⚠️  警告：此操作将彻底删除所有小程序及其相关数据！"
echo "包括："
echo "  - 所有小程序"
echo "  - 所有用户"
echo "  - 所有消息"
echo "  - 所有分配关系"
echo ""
read -p "确认要继续吗？(输入 yes 继续): " confirm

if [ "$confirm" != "yes" ]; then
    echo "操作已取消"
    exit 0
fi

# 获取 MySQL 容器名称
MYSQL_CONTAINER="h5-mysql"
DB_NAME="customer_service_db"
DB_USER="root"
DB_PASSWORD="rootpass"

echo "正在连接数据库..."

# 执行清除操作（使用 MYSQL_PWD 环境变量避免密码警告）
docker exec -i -e MYSQL_PWD=$DB_PASSWORD $MYSQL_CONTAINER mysql -u$DB_USER $DB_NAME <<EOF
-- 删除所有消息
DELETE FROM messages;

-- 删除所有用户
DELETE FROM users;

-- 删除所有分配关系
DELETE FROM assignments;

-- 删除所有小程序
DELETE FROM mini_apps;

-- 显示删除后的数据量
SELECT 
    (SELECT COUNT(*) FROM messages) as messages_count,
    (SELECT COUNT(*) FROM users) as users_count,
    (SELECT COUNT(*) FROM assignments) as assignments_count,
    (SELECT COUNT(*) FROM mini_apps) as miniapps_count;
EOF

echo ""
echo "✅ 清除完成！"

