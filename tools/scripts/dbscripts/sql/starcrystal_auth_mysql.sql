-- StarCrystal 认证库 + 激励广告 + 福利月末兑换日志 — 完整 DDL（DROP + CREATE）。MySQL 8.0+
-- 主键为业务键（VARCHAR / watch_id 等）；除 welfare_exchange_log.id 外无代理主键。
-- 经济字段 v7.1：curtoken / totaltoken（无 curmoney / totalmoney）；不兼容旧库，请整库重建。
--
-- mysql -u USER -p starcrystal_auth < tools/scripts/dbscripts/sql/starcrystal_auth_mysql.sql
--
SET NAMES utf8mb4;

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS welfare_exchange_log;
DROP TABLE IF EXISTS auth_invite_contrib_log;
DROP TABLE IF EXISTS auth_ad_completions;
DROP TABLE IF EXISTS auth_ad_watch_sessions;
DROP TABLE IF EXISTS auth_invite_members;
DROP TABLE IF EXISTS auth_invite_codes;
DROP TABLE IF EXISTS auth_device_account_map;
DROP TABLE IF EXISTS auth_accounts;

-- ---------------------------------------------------------------------------
-- 1) 主表：auth_accounts
-- ---------------------------------------------------------------------------
CREATE TABLE auth_accounts (
  account_id VARCHAR(191) NOT NULL COMMENT '账号类型_account_value 拼接，业务主键',
  account_type ENUM('phone','email','google','facebook','guest') NOT NULL COMMENT '账号类别，与 account_id 前缀一致',
  account_value VARCHAR(191) NOT NULL COMMENT '该类下的账号主体（邮箱、电话、OAuth sub 等），与类型共同决定 account_id',
  email VARCHAR(191) NULL,
  phone VARCHAR(64) NULL,
  invited_user_id VARCHAR(191) NULL COMMENT '邀请人用户ID（仅首次新建账号时写入）',
  password_hash VARCHAR(255) NOT NULL,
  device_id VARCHAR(191) NULL,
  registration_ip VARCHAR(45) NULL COMMENT '注册时上报的客户端 IP，用于同日多账号检测',
  ad_rewards_disabled TINYINT NOT NULL DEFAULT 0 COMMENT '1=可登录但不发激励广告奖励',
  fingerprint VARCHAR(255) NULL,
  provider VARCHAR(32) NOT NULL DEFAULT 'password',
  nickname VARCHAR(128) NULL,
  display_name VARCHAR(128) NULL,
  curgold DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '当前待兑换金币（月末批处理前）',
  totalgold DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '历史总金币（自赚累计，v7.2）',
  curtoken DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '当前 CurToken（待兑换礼品）',
  totaltoken DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '已兑换为礼品 Token 累计',
  cur_direct_inviter_share DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '当前直接上级分成',
  total_direct_inviter_share DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '总直接上级分成',
  cur_second_inviter_share DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '当前上级上级分成',
  total_second_inviter_share DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '总上级上级分成',
  cur_downline_l1_contrib DECIMAL(18,2) NOT NULL DEFAULT 0,
  total_downline_l1_contrib DECIMAL(18,2) NOT NULL DEFAULT 0,
  cur_downline_l2_contrib DECIMAL(18,2) NOT NULL DEFAULT 0,
  total_downline_l2_contrib DECIMAL(18,2) NOT NULL DEFAULT 0,
  notify_watermark_downline_l1 DECIMAL(18,2) NOT NULL DEFAULT 0,
  notify_watermark_downline_l2 DECIMAL(18,2) NOT NULL DEFAULT 0,
  notify_pending_since DATETIME NULL DEFAULT NULL,
  gold_balance DECIMAL(18,2) NOT NULL DEFAULT 0,
  cashout_last_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
  cashout_total_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1,
  ban_reason VARCHAR(255) NULL COMMENT '封号原因（status=2 时返回给客户端）',
  device_silent_until DATETIME NULL COMMENT '同设备新号注销确认后的静默截止时间（6小时后可触发封号）',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  PRIMARY KEY (account_id),
  KEY idx_account_value (account_value),
  KEY idx_email (email),
  KEY idx_phone (phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 1b) invite share audit log (v1.0.15 P0)
-- ---------------------------------------------------------------------------
CREATE TABLE auth_invite_contrib_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  earner_account_id VARCHAR(191) NOT NULL COMMENT 'earner account_id',
  beneficiary_account_id VARCHAR(191) NOT NULL COMMENT 'beneficiary account_id or platform',
  layer TINYINT NOT NULL COMMENT '1=direct 2=second',
  grant_kind VARCHAR(16) NOT NULL DEFAULT 'player' COMMENT 'player|gm',
  base_gold DECIMAL(18,2) NOT NULL COMMENT 'grantedDelta',
  raw_share DECIMAL(18,2) NOT NULL COMMENT 'theoretical share',
  paid_share DECIMAL(18,2) NOT NULL COMMENT 'capped actual payout',
  platform_share DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT 'raw-paid plus cap overflow',
  biz_type VARCHAR(64) NULL,
  biz_no VARCHAR(128) NULL,
  earner_nickname VARCHAR(128) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_beneficiary_time (beneficiary_account_id, created_at),
  KEY idx_earner_time (earner_account_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ---------------------------------------------------------------------------
-- 2) 设备-账号映射表：auth_device_account_map
-- ---------------------------------------------------------------------------
CREATE TABLE auth_device_account_map (
  device_id VARCHAR(191) NOT NULL COMMENT '客户端设备标识（deviceId）；一个设备可关联多个账号',
  account_id VARCHAR(191) NOT NULL COMMENT '账号业务主键（account_id）；同设备可有多行',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (device_id, account_id),
  KEY idx_device_created (device_id, created_at),
  KEY idx_account_id (account_id),
  CONSTRAINT fk_device_map_account_id
    FOREIGN KEY (account_id) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 3) 短邀请码映射表：auth_invite_codes（主键 invite_code；每账号唯一一行）
-- ---------------------------------------------------------------------------
CREATE TABLE auth_invite_codes (
  invite_code VARCHAR(16) NOT NULL COMMENT '服务端生成 9 位邀请码：2-9 与 A-Z 去掉易混 0/O/1/I（仅大写，QR/deeplink 载荷）',
  account_id VARCHAR(191) NOT NULL COMMENT '账号ID（auth_accounts.account_id）',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (invite_code),
  UNIQUE KEY uk_invite_codes_account_id (account_id),
  CONSTRAINT fk_invite_code_account_id
    FOREIGN KEY (account_id) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 4) 邀请关系表：auth_invite_members
-- ---------------------------------------------------------------------------
CREATE TABLE auth_invite_members (
  accountid VARCHAR(191) NOT NULL COMMENT '邀请人账号ID（inviteduserid）',
  inviteaccountid VARCHAR(191) NOT NULL COMMENT '被邀请新账号ID（新注册 account_id）',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (accountid, inviteaccountid),
  KEY idx_inviter (accountid),
  KEY idx_invitee (inviteaccountid),
  CONSTRAINT fk_invitee_account_id
    FOREIGN KEY (inviteaccountid) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 5) 激励广告：观看会话（watchId）与完成流水
-- ---------------------------------------------------------------------------
CREATE TABLE auth_ad_watch_sessions (
  watch_id CHAR(32) NOT NULL COMMENT '单次观看令牌，传给 H5：complete 必填',
  account_id VARCHAR(191) NOT NULL,
  slot VARCHAR(64) NOT NULL DEFAULT '',
  expires_at DATETIME NOT NULL COMMENT '过期后不可用',
  consumed TINYINT NOT NULL DEFAULT 0 COMMENT '0=未核销 1=已核销',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (watch_id),
  KEY idx_account_expires (account_id, expires_at),
  CONSTRAINT fk_ad_watch_account
    FOREIGN KEY (account_id) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE auth_ad_completions (
  account_id VARCHAR(191) NOT NULL,
  watch_id CHAR(32) NOT NULL COMMENT '与会话一对一，防重复领奖；业务主键',
  slot VARCHAR(64) NOT NULL DEFAULT '',
  completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (watch_id),
  KEY idx_account_day (account_id, completed_at),
  CONSTRAINT fk_ad_comp_account
    FOREIGN KEY (account_id) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 月末金币批量兑换日志（v7.1）
-- ---------------------------------------------------------------------------
CREATE TABLE welfare_exchange_log (
  id BIGINT NOT NULL AUTO_INCREMENT,
  account_id VARCHAR(191) NOT NULL,
  yyyymm CHAR(6) NOT NULL COMMENT '结算月 YYYYMM',
  gold_spent DECIMAL(18,2) NOT NULL DEFAULT 0,
  token_delta DECIMAL(18,2) NOT NULL DEFAULT 0,
  rate DECIMAL(18,8) NOT NULL DEFAULT 0,
  server_delta_snapshot DECIMAL(18,2) NOT NULL DEFAULT 0,
  rule_version VARCHAR(16) NOT NULL DEFAULT 'v7.1',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_account_yyyymm (account_id, yyyymm),
  CONSTRAINT fk_welfare_exchange_account
    FOREIGN KEY (account_id) REFERENCES auth_accounts(account_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;
