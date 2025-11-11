-- 为 mini_apps 表添加 name 字段（如果不存在）
-- 用于已存在的数据库迁移

USE customer_service_db;

-- 检查并添加 name 字段
SET @dbname = DATABASE();
SET @tablename = 'mini_apps';
SET @columnname = 'name';
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tablename)
      AND (table_schema = @dbname)
      AND (column_name = @columnname)
  ) > 0,
  'SELECT 1',
  CONCAT('ALTER TABLE ', @tablename, ' ADD COLUMN ', @columnname, ' VARCHAR(255) AFTER id')
));
PREPARE alterIfNotExists FROM @preparedStatement;
EXECUTE alterIfNotExists;
DEALLOCATE PREPARE alterIfNotExists;

SELECT 'name 字段已添加（如果之前不存在）' AS result;

