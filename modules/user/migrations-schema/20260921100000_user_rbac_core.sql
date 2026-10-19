-- +goose Up
-- RBAC 核心四表(user_ 前缀)。列与源账号服务存量表列兼容:
--   user_roles          <- roles
--   user_features       <- features(含 module/action/kind 维度列,与源表加列后一致)
--   user_role_features  <- role_features
--   user_users          <- users(含 totp_secret/totp_enabled/totp_recovery_codes)
-- 仅建空表,默认角色/feature/授权数据由后续数据迁移写入。

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_roles (
    id       BIGINT       PRIMARY KEY AUTO_INCREMENT,
    name     VARCHAR(255) NOT NULL,
    created  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_name (name),
    KEY idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_features (
    id       BIGINT       PRIMARY KEY AUTO_INCREMENT,
    name     VARCHAR(255) NOT NULL,
    module   VARCHAR(64)  NOT NULL DEFAULT '',
    action   VARCHAR(16)  NOT NULL DEFAULT '',
    kind     VARCHAR(16)  NOT NULL DEFAULT 'api',
    created  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_name (name),
    KEY idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_role_features (
    id         BIGINT   PRIMARY KEY AUTO_INCREMENT,
    role_id    BIGINT   NOT NULL,
    feature_id BIGINT   NOT NULL,
    created    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_role_feature (role_id, feature_id),
    KEY idx_role_id (role_id),
    KEY idx_feature_id (feature_id),
    CONSTRAINT fk_urf_role FOREIGN KEY (role_id) REFERENCES user_roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_urf_feature FOREIGN KEY (feature_id) REFERENCES user_features(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_users (
    id                  BIGINT       PRIMARY KEY AUTO_INCREMENT,
    email               VARCHAR(255) NOT NULL,
    password            VARCHAR(255) NOT NULL,
    role_id             BIGINT       NOT NULL,
    status              INT          NOT NULL DEFAULT 0,
    -- 两步验证(TOTP)。totp_secret 为加密密文(空=未绑定);totp_enabled setup 后 0 待确认,
    -- confirm 通过置 1;totp_recovery_codes 存备用码哈希 JSON。
    totp_secret         VARCHAR(255) NOT NULL DEFAULT '',
    totp_enabled        TINYINT(1)   NOT NULL DEFAULT 0,
    totp_recovery_codes TEXT,
    created             DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_email (email),
    KEY idx_email (email),
    KEY idx_role_id (role_id),
    KEY idx_status (status),
    KEY idx_created (created),
    CONSTRAINT fk_user_role FOREIGN KEY (role_id) REFERENCES user_roles(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_users;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS user_role_features;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS user_features;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS user_roles;
-- +goose StatementEnd
