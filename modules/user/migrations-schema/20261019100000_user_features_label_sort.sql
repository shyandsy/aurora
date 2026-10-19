-- user_features 增两列(供「可授权页目录」DB 派生用):
--   label_i18n JSON  —— 多语言显示名,如 {"zh-CN":"客户","zh-TW":"客戶","en":"Customers"};
--                       只对 ui.page.* 行有意义,其余 NULL。空值用 NULL(MySQL JSON 列拒收空串)。
--   sort_order INT   —— 页目录显示顺序(默认 0)。
-- 属 aurora 的 schema 套(user_schema_goose_db_version):结构变更归 aurora、恒先于项目 data 套执行。
-- 列定义(机制)归 aurora;列里的值(哪些页、各语言名、顺序)归各项目 data 套迁移 seed。
-- goose 单语句一块:一条 ALTER 同时加两列。

-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_features
    ADD COLUMN label_i18n JSON NULL,
    ADD COLUMN sort_order INT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_features
    DROP COLUMN label_i18n,
    DROP COLUMN sort_order;
-- +goose StatementEnd
