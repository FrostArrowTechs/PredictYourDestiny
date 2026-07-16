package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// AdminUserHandler handles admin user management endpoints.
type AdminUserHandler struct {
	DB *gorm.DB
}

// AdminUserListResponse is the paginated user list response.
type AdminUserListResponse struct {
	Users []AdminUserPayload `json:"users"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

// AdminUserPayload is the user info returned in admin responses.
type AdminUserPayload struct {
	ID          uint       `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	Role        string     `json:"role"`
	TierCode    string     `json:"tierCode"`
	TierName    string     `json:"tierName"`
	CreatedAt   string     `json:"createdAt"`
	LastActive  *string    `json:"lastActive"`
}

// ListUsers returns a paginated list of users with optional filters.
// Query params: page, limit, search, role, tier.
func (h *AdminUserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := h.DB.Model(&model.User{})

	// Optional search filter (email or display name)
	if search := c.Query("search"); search != "" {
		query = query.Where("email ILIKE ? OR display_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Optional role filter
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	query.Count(&total)

	var users []model.User
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}

	// Build response with tier info
	payload := make([]AdminUserPayload, len(users))
	for i, u := range users {
		payload[i] = AdminUserPayload{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		// Get user's membership tier
		var membership model.UserMembership
		if err := h.DB.Preload("Tier").Where("user_id = ?", u.ID).First(&membership).Error; err == nil {
			if membership.Tier != nil {
				payload[i].TierCode = membership.Tier.Code
				payload[i].TierName = membership.Tier.Name
			}
		}
	}

	c.JSON(http.StatusOK, AdminUserListResponse{
		Users: payload,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetUser returns detailed info about a single user.
func (h *AdminUserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var user model.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Get membership tier
	var membership model.UserMembership
	tierCode := ""
	tierName := ""
	if err := h.DB.Preload("Tier").Where("user_id = ?", user.ID).First(&membership).Error; err == nil {
		if membership.Tier != nil {
			tierCode = membership.Tier.Code
			tierName = membership.Tier.Name
		}
	}

	// Get today's quota usage
	today := time.Now().UTC().Format("2006-01-02")
	var quota model.UsageQuota
	todayUsed := 0
	if err := h.DB.Where("user_id = ? AND date = ?", user.ID, today).First(&quota).Error; err == nil {
		todayUsed = quota.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"user": AdminUserPayload{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			TierCode:    tierCode,
			TierName:    tierName,
			CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		"stats": gin.H{
			"todayUsed": todayUsed,
		},
	})
}

// UpdateUserRoleRequest is the payload for PUT /api/admin/users/:id/role.
type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin"`
}

// UpdateUserRole changes a user's role (user/admin).
func (h *AdminUserHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user.Role = req.Role
	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// UpdateUserTierRequest is the payload for PUT /api/admin/users/:id/tier.
type UpdateUserTierRequest struct {
	TierID uint `json:"tierId" binding:"required"`
}

// UpdateUserTier changes a user's membership tier.
func (h *AdminUserHandler) UpdateUserTier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateUserTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tier exists
	var tier model.MembershipTier
	if err := h.DB.First(&tier, req.TierID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tier id"})
		return
	}

	// Upsert membership
	var membership model.UserMembership
	err = h.DB.Where("user_id = ?", id).First(&membership).Error

	if err == gorm.ErrRecordNotFound {
		// Create new membership
		membership = model.UserMembership{
			UserID: uint(id),
			TierID: req.TierID,
		}
		if err := h.DB.Create(&membership).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create membership"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch membership"})
		return
	} else {
		// Update existing membership
		membership.TierID = req.TierID
		if err := h.DB.Save(&membership).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update membership"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "tier updated"})
}