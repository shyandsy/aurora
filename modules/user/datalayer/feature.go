package datalayer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/entity"
	"gorm.io/gorm"
)

type FeatureDatalayer interface {
	GetByID(ctx context.Context, id int64) (*entity.Feature, error)
	GetByName(ctx context.Context, name string) (*entity.Feature, error)
	GetAll(ctx context.Context) ([]entity.Feature, error)
	GetByRoleID(ctx context.Context, roleID int64) ([]entity.Feature, error)
	// GetExpandedNamesByRoleID 读角色 features 并做「签发时展开」,返回进 token/展示用的扁平 feature name。
	GetExpandedNamesByRoleID(ctx context.Context, roleID int64) ([]string, error)
	Create(ctx context.Context, feature *entity.Feature) error
	Update(ctx context.Context, feature *entity.Feature) error
	Delete(ctx context.Context, id int64) error
}

// featureDatalayer 功能数据访问层
type featureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewFeatureDatalayer 创建功能数据访问层
func NewFeatureDatalayer(app contracts.App) FeatureDatalayer {
	dl := &featureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve FeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID 根据ID获取功能
func (d *featureDatalayer) GetByID(ctx context.Context, id int64) (*entity.Feature, error) {
	var feature entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.First(&feature, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &feature, nil
}

// GetByName 根据名称获取功能
func (d *featureDatalayer) GetByName(ctx context.Context, name string) (*entity.Feature, error) {
	var feature entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Where("name = ?", name).First(&feature).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &feature, nil
}

// GetAll 获取所有功能
func (d *featureDatalayer) GetAll(ctx context.Context) ([]entity.Feature, error) {
	var features []entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// GetByRoleID 根据角色ID获取功能列表
func (d *featureDatalayer) GetByRoleID(ctx context.Context, roleID int64) ([]entity.Feature, error) {
	var features []entity.Feature
	db := d.DB.WithContext(ctx)
	// 表名用 user_ 前缀(与 entity TableName 一致)。#50 拆分改了表名,漏改了这句裸 SQL → 登录发 JWT
	// 加载权限时查不存在的 features 表而 500(2FA 那步尤其明显)。
	if err := db.Table("user_features").
		Joins("INNER JOIN user_role_features ON user_features.id = user_role_features.feature_id").
		Where("user_role_features.role_id = ?", roleID).
		Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// GetExpandedNamesByRoleID 读角色直接持有的 features,并做「签发时展开」(expand-at-issue)。
// 展开三步闭包(通用 RBAC 富引擎,读 user_ 前缀表):
//  1. 角色 user_role_features 里直接持有的 feature name(含 ui.page.*、ui.menu.*、业务 api 都可能有);
//  2. 每个 ui.page.* → user_ui_feature_grants 指向的 feature(业务 api + 对应 ui.menu.*);
//  3. 对结果集里每个 feature 再并入 user_feature_dependencies 的传递闭包(如 document.create→storage.upload)。
//
// 若角色直授通配 feature "*"(超级管理员)→ 直接返回 ["*"],不逐条展开(中间件视其为全通)。
// 返回:去重、排序、且**剔除内部 ui.page.* 入口标记**后的扁平 name 列表(ui.page.* 只是展开钥匙,不进 token)。
// 展开只发生在这里(签发侧):JWT claims 仍是扁平 feature 列表,中间件精确匹配、aurora 校验一行不改。
//
// 逻辑通用、可复制;具体 feature/grants/dependencies 数据由每项目 seed 决定,不进代码。
func (d *featureDatalayer) GetExpandedNamesByRoleID(ctx context.Context, roleID int64) ([]string, error) {
	db := d.DB.WithContext(ctx)

	// 1. 角色直接持有的 feature name
	var rawNames []string
	if err := db.Raw(
		`SELECT f.name FROM user_features f
		 INNER JOIN user_role_features rf ON f.id = rf.feature_id
		 WHERE rf.role_id = ?`, roleID,
	).Scan(&rawNames).Error; err != nil {
		return nil, err
	}

	// 注意:通配 "*"(超级管理员)**不短路**,和 admin 一致——它作为普通 feature 名
	// 一路进入展开结果、留在 token 里。中间件按 includes("*") 全通;前端有的地方直接
	// getUserFeatures().includes("具体feature")(不走 hasFeature、不认 "*"),所以必须返回
	// 完整枚举列表而非 ["*"],否则超管反而看不到 dashboard 等按具体 feature 分区的区块。
	type namePair struct {
		Src string
		Dst string
	}

	// 2. user_ui_feature_grants:ui feature name → 授予的 feature name
	grantMap := make(map[string][]string)
	{
		var rows []namePair
		if err := db.Raw(
			`SELECT ui.name AS src, t.name AS dst
			 FROM user_ui_feature_grants g
			 INNER JOIN user_features ui ON ui.id = g.ui_feature_id
			 INNER JOIN user_features t ON t.id = g.granted_feature_id`,
		).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			grantMap[r.Src] = append(grantMap[r.Src], r.Dst)
		}
	}

	// 3. user_feature_dependencies:feature name → 依赖的 feature name
	depMap := make(map[string][]string)
	{
		var rows []namePair
		if err := db.Raw(
			`SELECT f.name AS src, r.name AS dst
			 FROM user_feature_dependencies d
			 INNER JOIN user_features f ON f.id = d.feature_id
			 INNER JOIN user_features r ON r.id = d.requires_feature_id`,
		).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			depMap[r.Src] = append(depMap[r.Src], r.Dst)
		}
	}

	// 闭包展开:grant + dependency 都做传递闭包(worklist)。
	seen := make(map[string]bool)
	// ui.menu.* 只允许由 ui.page.*.view/operate 经 ui_feature_grants 展开派生;
	// 直接授予的裸 ui.menu.*(历史遗留/误授)一律忽略——否则会绕过页面档位直接驱动菜单。
	queue := make([]string, 0, len(rawNames))
	for _, n := range rawNames {
		if strings.HasPrefix(n, "ui.menu.") {
			continue
		}
		queue = append(queue, n)
	}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		for _, t := range grantMap[n] {
			if !seen[t] {
				queue = append(queue, t)
			}
		}
		for _, r := range depMap[n] {
			if !seen[r] {
				queue = append(queue, r)
			}
		}
	}

	// 剔除内部 ui.page.* 入口标记(不进 token),排序输出保证稳定。
	out := make([]string, 0, len(seen))
	for n := range seen {
		if strings.HasPrefix(n, "ui.page.") {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// Create 创建功能
func (d *featureDatalayer) Create(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Create(feature).Error
}

// Update 更新功能
func (d *featureDatalayer) Update(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Save(feature).Error
}

// Delete 删除功能
func (d *featureDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Feature{}, id).Error
}
