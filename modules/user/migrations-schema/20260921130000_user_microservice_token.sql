-- +goose Up
-- 微服务 token 管理（user_ 前缀)。列与源存量表列兼容,便于后续数据迁移 INSERT...SELECT 拷贝存量:
--   user_microservice_token_features        <- microservice_token_features
--   user_microservice_token_features_token  <- microservice_token_features_token
-- 仅建空表,默认 microservice token 数据由后续数据迁移/运营写入。
-- 两张表均无外键:token 表的 microservice_feature_ids 为 JSON 数组而非 FK,故无跨表迁移顺序约束。

-- +goose StatementBegin
-- 预定义的微服务 token 功能配置（每条 = 一个微服务身份 + 其所需 feature 列表）。
CREATE TABLE IF NOT EXISTS user_microservice_token_features (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name         VARCHAR(255)    NOT NULL COMMENT '微服务身份标识（如 schedule_worker_client)',
    description  VARCHAR(500)    NOT NULL COMMENT '微服务 token 用途描述',
    feature_list JSON            NOT NULL COMMENT '该微服务所需 feature 名称数组',
    created      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    modified     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_name (name),
    KEY idx_created (created)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose StatementBegin
-- 已签发的微服务 token 记录（含黑名单启用/禁用状态）。
CREATE TABLE IF NOT EXISTS user_microservice_token_features_token (
    id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    microservice_feature_ids JSON            NOT NULL COMMENT '生成本 token 所用的功能配置 ID 数组',
    description              TEXT            COMMENT '所选微服务描述合并',
    feature_list            JSON            NOT NULL COMMENT '所选微服务 feature 合并列表',
    token                   TEXT            NOT NULL COMMENT '签发的 JWT token',
    issuer                  VARCHAR(255)    NOT NULL COMMENT 'JWT 签发者',
    expires_at              DATETIME        NOT NULL COMMENT 'token 过期时间',
    expires_in              BIGINT          NOT NULL COMMENT 'token 有效期（秒)',
    status                  VARCHAR(20)     NOT NULL DEFAULT 'ENABLED' COMMENT 'token 状态: ENABLED 或 DISABLED',
    created                 DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    modified                DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    PRIMARY KEY (id),
    KEY idx_expires_at (expires_at),
    KEY idx_created (created),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_microservice_token_features_token;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS user_microservice_token_features;
-- +goose StatementEnd
