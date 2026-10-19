-- +goose Up
-- UI 富展开授权 + feature 依赖闭包(user_ 前缀)。列与源存量表列兼容:
--   user_ui_feature_grants   <- ui_feature_grants
--   user_feature_dependencies <- feature_dependencies
-- 仅建空表,具体授权/依赖数据由后续数据迁移写入。

-- +goose StatementBegin
-- UI feature -> 授予的 feature(可为业务 api,也可为 ui.menu.*,展开时一并进 token)。
CREATE TABLE IF NOT EXISTS user_ui_feature_grants (
    id                 BIGINT      NOT NULL AUTO_INCREMENT,
    ui_feature_id      BIGINT      NOT NULL,
    granted_feature_id BIGINT      NOT NULL,
    tier               VARCHAR(16) NOT NULL DEFAULT '',
    created            DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_ui_grant (ui_feature_id, granted_feature_id),
    KEY idx_ui_feature (ui_feature_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose StatementBegin
-- feature 级依赖:授了 feature_id 必须连带授 requires_feature_id。
CREATE TABLE IF NOT EXISTS user_feature_dependencies (
    id                  BIGINT   NOT NULL AUTO_INCREMENT,
    feature_id          BIGINT   NOT NULL,
    requires_feature_id BIGINT   NOT NULL,
    created             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_feat_dep (feature_id, requires_feature_id),
    KEY idx_feature (feature_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_feature_dependencies;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS user_ui_feature_grants;
-- +goose StatementEnd
