-- +goose Up
-- 登录会话/设备清单:一次登录 = 一行(refresh 续期算同一会话,session_id 跨 refresh 不变)。
-- 支撑「我的登录设备」清单 + 一键撤销。web 与 app 共用此表(client_type 区分)。
-- 撤销机制:标 revoked + 删除该会话当前 access/refresh jti 的 tokenip 绑定(fail-close 即时失效)。
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_session (
    id              BIGINT       PRIMARY KEY AUTO_INCREMENT,
    session_id      VARCHAR(64)  NOT NULL COMMENT '会话稳定标识(uuid,跨 refresh 不变)',
    user_id         BIGINT       NOT NULL,
    client_type     VARCHAR(16)  NOT NULL DEFAULT 'web' COMMENT 'web / app',
    device_name     VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备/浏览器',
    user_agent      VARCHAR(512) NOT NULL DEFAULT '',
    login_ip        VARCHAR(64)  NOT NULL DEFAULT '',
    last_ip         VARCHAR(64)  NOT NULL DEFAULT '',
    login_geo       VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'IP 归属地',
    last_geo        VARCHAR(128) NOT NULL DEFAULT '',
    cur_access_jti  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '当前有效 access token 的 jti',
    cur_refresh_jti VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '当前有效 refresh token 的 jti',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '登录时间',
    last_seen_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近活跃(refresh 时刷新)',
    expires_at      DATETIME     NULL COMMENT '会话硬上限(= refresh 过期时间)',
    revoked         TINYINT(1)   NOT NULL DEFAULT 0,
    revoked_at      DATETIME     NULL,
    revoked_reason  VARCHAR(64)  NOT NULL DEFAULT '',
    UNIQUE KEY uk_session_id (session_id),
    KEY idx_user_active (user_id, revoked),
    KEY idx_refresh_jti (cur_refresh_jti)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT '登录会话/设备清单(web+app;可列出+撤销)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_session;
-- +goose StatementEnd
