-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS roles (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL UNIQUE,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (name)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Insert default roles: admin, user
INSERT INTO roles (name) VALUES ('admin'), ('user');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS features (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL UNIQUE,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (name)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Insert default features: user, role, feature, rolefeature CRUD operations
INSERT INTO features (name) VALUES 
    ('user.get'),
    ('user.update'),
    ('user.create'),
    ('user.delete'),
    ('role.get'),
    ('role.create'),
    ('role.update'),
    ('role.delete'),
    ('feature.get'),
    ('feature.create'),
    ('feature.update'),
    ('feature.delete'),
    ('rolefeature.get'),
    ('rolefeature.create'),
    ('rolefeature.delete'),
    ('customer.get'),
    ('customer.create'),
    ('customer.update'),
    ('customer.delete');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS role_features (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    role_id BIGINT NOT NULL,
    feature_id BIGINT NOT NULL,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
    UNIQUE KEY uk_role_feature (role_id, feature_id),
    INDEX idx_role_id (role_id),
    INDEX idx_feature_id (feature_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Insert default role-feature mappings: admin role has all features
INSERT INTO role_features (role_id, feature_id)
SELECT r.id, f.id
FROM roles r
CROSS JOIN features f
WHERE r.name = 'admin'
AND f.name IN (
    'user.get', 'user.update', 'user.create', 'user.delete',
    'role.get', 'role.create', 'role.update', 'role.delete',
    'feature.get', 'feature.create', 'feature.update', 'feature.delete',
    'rolefeature.get', 'rolefeature.create', 'rolefeature.delete',
    'customer.get', 'customer.create', 'customer.update', 'customer.delete'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role_id BIGINT NOT NULL,
    status INT NOT NULL DEFAULT 0,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT,
    INDEX idx_email (email),
    INDEX idx_role_id (role_id),
    INDEX idx_status (status),
    INDEX idx_created (created)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Insert default admin user: shyandsy@gmail.com / 123456
-- 关联到 admin role
INSERT INTO users (email, password, role_id, status) VALUES (
    'shyandsy@gmail.com',
    '$2a$10$rHmpuJbXkk9bn/1.FaGyvOR7O4A2sye9N2aJ08J/dDjw7CrJ5qTK6',
    (SELECT id FROM roles WHERE name = 'admin' LIMIT 1),
    1
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS role_features;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS features;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS roles;
-- +goose StatementEnd
