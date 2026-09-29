-- +goose Up
-- ============================================================================
-- homeserver 专用存量数据采纳迁移(复制本 user 服务到新项目时,删除此文件)。
-- ----------------------------------------------------------------------------
-- 唯一非通用、项目专用的迁移:同库把 admin 现有账号/角色/feature/授权/microservice
-- token 数据整列拷进 commit 4/5 建好的 user_ 空表。admin 老表保持不动、继续服务
-- (并行运行,cutover 在最后)。
--
-- 关键约束:
--  1. 每条 INSERT 显式列出列名并带 id,整列直拷、保持 id 不变——role_features /
--     users / ui_feature_grants 靠 id 关联;microservice token 表的
--     microservice_feature_ids 是指向 features.id 的 JSON 数组,id 变了数组就指错行。
--  2. INSERT IGNORE:按主键/唯一键跳过已存在行,迁移可安全重跑、不炸重复键
--     (存量与运营新增可反复采纳)。
--  3. FK 安全顺序:父表(roles/features)先于子表(role_features/users)。
--  4. user_session 不拷:会话是活的、重登即生,存量快照无意义。
--  5. microservice.* 权限一并带过:features 含 microservice.* 行、role_features /
--     ui_feature_grants 含对应授权,拷这三张即把 microservice 路由的权限 gate 迁过来
--     (解决 commit 5「路由 403 直到 seed」的 TODO)。
--
-- 说明:此迁移引用 admin 源表名(users/roles/features/... 无前缀)是采纳设计使然。
-- ============================================================================

-- +goose StatementBegin
-- 1. roles -> user_roles
INSERT IGNORE INTO user_roles (id, name, created, modified)
SELECT id, name, created, modified FROM roles;
-- +goose StatementEnd

-- +goose StatementBegin
-- 2. features -> user_features(含 module/action/kind 维度列)
INSERT IGNORE INTO user_features (id, name, module, action, kind, created, modified)
SELECT id, name, module, action, kind, created, modified FROM features;
-- +goose StatementEnd

-- +goose StatementBegin
-- 3. users -> user_users(status 源为 INT,兼容;含 totp 三列)
INSERT IGNORE INTO user_users
    (id, email, password, role_id, status, totp_secret, totp_enabled, totp_recovery_codes, created, modified)
SELECT id, email, password, role_id, status, totp_secret, totp_enabled, totp_recovery_codes, created, modified
FROM users;
-- +goose StatementEnd

-- +goose StatementBegin
-- 4. role_features -> user_role_features(role_id/feature_id 靠上面保持不变的 id 关联)
INSERT IGNORE INTO user_role_features (id, role_id, feature_id, created, modified)
SELECT id, role_id, feature_id, created, modified FROM role_features;
-- +goose StatementEnd

-- +goose StatementBegin
-- 5. ui_feature_grants -> user_ui_feature_grants(microservice UI 授权闭包随此拷过来)
INSERT IGNORE INTO user_ui_feature_grants (id, ui_feature_id, granted_feature_id, tier, created)
SELECT id, ui_feature_id, granted_feature_id, tier, created FROM ui_feature_grants;
-- +goose StatementEnd

-- +goose StatementBegin
-- 6. feature_dependencies -> user_feature_dependencies
INSERT IGNORE INTO user_feature_dependencies (id, feature_id, requires_feature_id, created)
SELECT id, feature_id, requires_feature_id, created FROM feature_dependencies;
-- +goose StatementEnd

-- +goose StatementBegin
-- 7. microservice_token_features -> user_microservice_token_features
INSERT IGNORE INTO user_microservice_token_features (id, name, description, feature_list, created, modified)
SELECT id, name, description, feature_list, created, modified FROM microservice_token_features;
-- +goose StatementEnd

-- +goose StatementBegin
-- 8. microservice_token_features_token -> user_microservice_token_features_token
--    microservice_feature_ids(JSON 数组,指向 features.id)连 id 一起拷、保持不变。
INSERT IGNORE INTO user_microservice_token_features_token
    (id, microservice_feature_ids, description, feature_list, token, issuer, expires_at, expires_in, status, created, modified)
SELECT id, microservice_feature_ids, description, feature_list, token, issuer, expires_at, expires_in, status, created, modified
FROM microservice_token_features_token;
-- +goose StatementEnd

-- +goose Down
-- 采纳的逆操作 = 清空 user_ 表数据(表由 commit 4/5 建,DROP 归它们的 down)。
-- 顺序与 up 相反:先删子表,再删父表。

-- +goose StatementBegin
DELETE FROM user_microservice_token_features_token;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_microservice_token_features;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_feature_dependencies;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_ui_feature_grants;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_role_features;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_users;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_features;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM user_roles;
-- +goose StatementEnd
