-- 首次初始化数据目录时创建库与账号（升级 schema 仍由 docker_svrdev / rebuild-auth-mysql 负责）
CREATE DATABASE IF NOT EXISTS starcrystal_auth
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'star_auth'@'%' IDENTIFIED BY 'star_auth_123456';
CREATE USER IF NOT EXISTS 'star_auth'@'localhost' IDENTIFIED BY 'star_auth_123456';
GRANT ALL PRIVILEGES ON starcrystal_auth.* TO 'star_auth'@'%';
GRANT ALL PRIVILEGES ON starcrystal_auth.* TO 'star_auth'@'localhost';
FLUSH PRIVILEGES;
