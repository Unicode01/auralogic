package repository

import (
	"auralogic/internal/models"
	"gorm.io/gorm"
	"strings"
)

type UserRepository struct {
	db *gorm.DB
}

type UserListFilters struct {
	Search               string
	Role                 string
	IsActive             *bool
	EmailVerified        *bool
	EmailNotifyMarketing *bool
	SMSNotifyMarketing   *bool
	HasPhone             *bool
	Locale               string
	Country              string
}

// ListCountries returns distinct, normalized country codes that exist on users.
func (r *UserRepository) ListCountries(role string) ([]string, error) {
	countries := make([]string, 0)
	query := r.db.Model(&models.User{}).
		Where("country IS NOT NULL").
		Where("TRIM(country) <> ''")

	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "all":
	case "user", "users":
		query = query.Where("role = ?", "user")
	case "admin", "admins":
		query = query.Where("role IN ?", []string{"admin", "super_admin"})
	case "super_admin":
		query = query.Where("role = ?", "super_admin")
	default:
		query = query.Where("role = ?", role)
	}

	if err := query.
		Select("DISTINCT UPPER(TRIM(country)) AS country").
		Order("country ASC").
		Pluck("country", &countries).Error; err != nil {
		return nil, err
	}

	return countries, nil
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 创建用户
func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// FindByID 根据ID查找用户
func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据Email查找用户
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone 根据Phone查找用户
func (r *UserRepository) FindByPhone(phone string) (*models.User, error) {
	var user models.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUUID 根据UUID查找用户
func (r *UserRepository) FindByUUID(uuid string) (*models.User, error) {
	var user models.User
	err := r.db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 更新用户
func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// UpdateConsumptionStats 更新用户消费统计
func (r *UserRepository) UpdateConsumptionStats(userID uint, totalSpentMinor int64, totalOrderCount int64) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"total_spent_minor": totalSpentMinor,
			"total_order_count": totalOrderCount,
		}).Error
}

// Delete 删除用户（软删除）
func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// List 获取用户列表
func (r *UserRepository) List(page, limit int, filters UserListFilters) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.db.Model(&models.User{})

	search := strings.TrimSpace(filters.Search)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("email LIKE ? OR name LIKE ? OR phone LIKE ?", like, like, like)
	}

	switch strings.ToLower(strings.TrimSpace(filters.Role)) {
	case "", "all":
	case "user", "users":
		query = query.Where("role = ?", "user")
	case "admin", "admins":
		query = query.Where("role IN ?", []string{"admin", "super_admin"})
	case "super_admin":
		query = query.Where("role = ?", "super_admin")
	default:
		query = query.Where("role = ?", filters.Role)
	}

	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}
	if filters.EmailVerified != nil {
		query = query.Where("email_verified = ?", *filters.EmailVerified)
	}
	if filters.EmailNotifyMarketing != nil {
		query = query.Where("email_notify_marketing = ?", *filters.EmailNotifyMarketing)
	}
	if filters.SMSNotifyMarketing != nil {
		query = query.Where("sms_notify_marketing = ?", *filters.SMSNotifyMarketing)
	}
	if filters.HasPhone != nil {
		if *filters.HasPhone {
			query = query.Where("phone IS NOT NULL AND phone <> ''")
		} else {
			query = query.Where("phone IS NULL OR phone = ''")
		}
	}

	locale := strings.TrimSpace(filters.Locale)
	if locale != "" {
		query = query.Where("LOWER(locale) = LOWER(?)", locale)
	}
	country := strings.TrimSpace(filters.Country)
	if country != "" {
		query = query.Where("LOWER(country) = LOWER(?)", country)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Find(&users).Error

	return users, total, err
}
