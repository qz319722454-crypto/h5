-- 清除所有小程序及其相关数据
-- 注意：此操作会彻底删除所有数据，无法恢复！
-- 使用前请确保已备份数据库

USE customer_service_db;

-- 方式1：硬删除（彻底删除，推荐用于清理测试数据）
-- 按依赖关系顺序删除

-- 1. 删除所有消息（因为消息依赖用户和客服）
DELETE FROM messages;

-- 2. 删除所有用户（因为用户依赖小程序）
DELETE FROM users;

-- 3. 删除所有分配关系（因为分配依赖小程序和客服）
DELETE FROM assignments;

-- 4. 删除所有小程序
DELETE FROM mini_apps;

-- 重置自增ID（可选，如果需要从1开始重新计数）
-- ALTER TABLE messages AUTO_INCREMENT = 1;
-- ALTER TABLE users AUTO_INCREMENT = 1;
-- ALTER TABLE assignments AUTO_INCREMENT = 1;
-- ALTER TABLE mini_apps AUTO_INCREMENT = 1;

-- ============================================
-- 方式2：软删除（如果使用 GORM 的软删除功能）
-- 注意：如果表中有 deleted_at 字段，GORM 默认使用软删除
-- 如果要彻底删除，需要使用上面的硬删除方式
-- ============================================

-- 查看删除后的数据量（验证）
SELECT 
    (SELECT COUNT(*) FROM messages) as messages_count,
    (SELECT COUNT(*) FROM users) as users_count,
    (SELECT COUNT(*) FROM assignments) as assignments_count,
    (SELECT COUNT(*) FROM mini_apps) as miniapps_count;


