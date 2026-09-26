-- +goose Up
-- doorman feature 的三张自持表(中性,doorman_ 前缀:doorman_rule / doorman_action_policy / doorman_decision)。
--
-- ⚠️ doorman 是「库/组件」不是「服务」——它自己没有进程、不跑迁移。此文件是它 schema 的**唯一真相源**,
--    随组件一起走。执行者永远是**嵌入 doorman 的宿主服务**的 goose runner。
--
-- 采用方(任何用 doorman 的项目)只需:①引用 Go feature(github.com/shyandsy/aurora/feature/doorman)
--    ②复制 doorman 配置页前端 ③把本文件复制进自己**跑 goose 的那个服务**的迁移目录(带上该项目的时间戳命名)。
--    多个服务共用一库时,让其中一个跑 goose 的服务建表即可;其余嵌 doorman 的服务只用表、不建表。
--
-- 幂等:CREATE TABLE IF NOT EXISTS —— 若表已存在(如历史上曾用 AutoMigrate 建过)则跳过,不报错。

CREATE TABLE IF NOT EXISTS doorman_rule (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  scope       VARCHAR(64)  NOT NULL DEFAULT '',
  name        VARCHAR(128) NOT NULL DEFAULT '',
  conditions  TEXT         NULL,
  combine     VARCHAR(8)   NOT NULL DEFAULT 'and',
  risk_level  VARCHAR(16)  NOT NULL DEFAULT 'none',
  enabled     TINYINT(1)   NOT NULL DEFAULT 0,
  created     DATETIME     NULL,
  modified    DATETIME     NULL,
  PRIMARY KEY (id),
  KEY idx_doorman_scope (scope)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doorman_action_policy (
  scope       VARCHAR(64) NOT NULL DEFAULT '',
  risk_level  VARCHAR(16) NOT NULL DEFAULT '',
  action      VARCHAR(64) NOT NULL DEFAULT '',
  modified    DATETIME    NULL,
  PRIMARY KEY (scope, risk_level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doorman_decision (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  scope       VARCHAR(64)  NOT NULL DEFAULT '',
  risk_level  VARCHAR(16)  NOT NULL DEFAULT '',
  action      VARCHAR(64)  NOT NULL DEFAULT '',
  matched     VARCHAR(512) NOT NULL DEFAULT '',
  ua          VARCHAR(512) NOT NULL DEFAULT '',
  ip          VARCHAR(45)  NOT NULL DEFAULT '',
  country     VARCHAR(8)   NOT NULL DEFAULT '',
  isp         VARCHAR(64)  NOT NULL DEFAULT '',
  asn         INT UNSIGNED NOT NULL DEFAULT 0,
  asn_org     VARCHAR(128) NOT NULL DEFAULT '',
  is_hosting  TINYINT(1)   NOT NULL DEFAULT 0,
  challenged  TINYINT(1)   NOT NULL DEFAULT 0,
  subject     VARCHAR(191) NOT NULL DEFAULT '',
  outcome     VARCHAR(32)  NOT NULL DEFAULT '',
  created     DATETIME     NULL,
  PRIMARY KEY (id),
  KEY idx_doorman_decision_scope_id (scope),
  KEY idx_doorman_decision_scope_subject (scope, subject),
  KEY idx_doorman_decision_created (created)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS doorman_decision;
DROP TABLE IF EXISTS doorman_action_policy;
DROP TABLE IF EXISTS doorman_rule;
