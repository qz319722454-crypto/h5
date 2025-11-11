-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS customer_service_db;
USE customer_service_db;

-- MiniApp 表
CREATE TABLE IF NOT EXISTS mini_apps (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    name VARCHAR(255),
    app_id VARCHAR(255) UNIQUE,
    secret VARCHAR(255),
    template_id VARCHAR(255)
);

-- CustomerService 表
CREATE TABLE IF NOT EXISTS customer_services (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    name VARCHAR(255) UNIQUE,
    password VARCHAR(255),
    is_admin TINYINT(1) DEFAULT 0,
    qr_code_path VARCHAR(500),
    welcome_message TEXT
);

-- Assignment 表
CREATE TABLE IF NOT EXISTS assignments (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    mini_app_id INT UNSIGNED,
    customer_service_id INT UNSIGNED
);

-- User 表
CREATE TABLE IF NOT EXISTS users (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    open_id VARCHAR(255) UNIQUE,
    mini_app_id INT UNSIGNED,
    subscribed TINYINT(1),
    last_active_time DATETIME
);

-- Message 表
CREATE TABLE IF NOT EXISTS messages (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    user_id INT UNSIGNED,
    customer_service_id INT UNSIGNED,
    content TEXT,
    from_user TINYINT(1),
    is_image TINYINT(1) DEFAULT 0,
    image_url VARCHAR(500),
    is_read TINYINT(1) DEFAULT 0
);

-- 可选初始数据（示例）
INSERT INTO mini_apps (app_id, secret, template_id) VALUES ('example-appid', 'example-secret', 'example-templateid');

-- 默认管理员账号: admin/admin
-- 密码 "admin" 的 bcrypt 哈希 (cost=10)
INSERT INTO customer_services (name, password, is_admin, created_at, updated_at) 
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE password = VALUES(password), is_admin = VALUES(is_admin);
