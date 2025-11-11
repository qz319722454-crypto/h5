#!/bin/bash

# 创建管理员账号脚本
# 用于快速创建或更新管理员账号

echo "创建管理员账号（admin/admin）..."

docker exec -i h5-mysql mysql -uroot -prootpass customer_service_db << 'EOF'
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

-- 插入或更新管理员账号
INSERT INTO customer_services (name, password, is_admin, created_at, updated_at) 
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE 
    password = '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    is_admin = 1,
    updated_at = NOW();

SELECT name, is_admin, created_at FROM customer_services WHERE name = 'admin';
EOF

echo "完成！现在可以使用 admin/admin 登录。"
