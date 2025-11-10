package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/app/model"
	apperr "github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/pkg/err"
	"github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/pkg/logger"
	"github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/app/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// --- 请求/响应 DTOs ---

type CreateWishRequest struct {
	Content    string   `json:"content" binding:"required"`
	IsPublic   *bool    `json:"isPublic"`
	Background *string  `json:"background"`
	Tags       []string `json:"tags"`
}

type WishResponse struct {
	ID           uint            `json:"id"`
	UserID       uint            `json:"userId"`
	Content      string          `json:"content"`
	IsPublic     bool            `json:"isPublic"`
	Background   string          `json:"background"`
	LikeCount    int             `json:"likeCount"`
	CommentCount int             `json:"commentCount"`
	CreatedAt    time.Time       `json:"createdAt"`
	User         UserResponse    `json:"user,omitempty"`
	Tags         []model.WishTag `json:"tags,omitempty"`
}

type LikeToggleResponse struct {
	Liked bool `json:"liked"`
}

// --- Helper ---

func wishToResponse(w model.Wish) WishResponse {
	resp := WishResponse{
		ID:           w.ID,
		UserID:       w.UserID,
		Content:      w.Content,
		IsPublic:     w.IsPublic,
		Background:   w.Background,
		LikeCount:    w.LikeCount,
		CommentCount: w.CommentCount,
		CreatedAt:    w.CreatedAt,
	}
	// user info if preloaded
	if w.User.ID != 0 {
		resp.User = UserResponse{
			ID:        w.User.ID,
			Username:  w.User.Username,
			Nickname:  w.User.Nickname,
			AvatarID:  w.User.AvatarID,
			Role:      w.User.Role,
			CreatedAt: w.User.CreatedAt,
		}
	}
	if len(w.Tags) > 0 {
		resp.Tags = w.Tags
	}
	return resp
}

// --- Handlers ---

// GetPublicWishes GET /api/wishes/public
// 支持分页参数 ?page=1&pageSize=20
func GetPublicWishes(c *gin.Context, db *gorm.DB) {
	// pagination
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")
	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var wishes []model.Wish
	offset := (page - 1) * pageSize

	if err := db.
		Where("is_public = ?", true).
		Preload("User").
		Preload("Tags").
		Order("created_at desc").
		Limit(pageSize).
		Offset(offset).
		Find(&wishes).Error; err != nil {
		logger.Log.Errorw("GetPublicWishes: DB 查询失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": apperr.GetMsg(apperr.ERROR_SERVER_ERROR),
			"data":    gin.H{},
		})
		return
	}

	var respList []WishResponse
	for _, w := range wishes {
		respList = append(respList, wishToResponse(w))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    apperr.SUCCESS,
		"message": "获取公开愿望成功",
		"data": gin.H{
			"wishes": respList,
			"page":   page,
		},
	})
}

// CreateWish POST /api/wishes
func CreateWish(c *gin.Context, db *gorm.DB) {
	var req CreateWishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warnw("CreateWish: 参数绑定失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_PARAM_INVALID,
			"message": "请检查输入的愿望内容",
			"data":    gin.H{"error": err.Error()},
		})
		return
	}

	// AI 内容审核（如果不安全则拒绝）
	isViolating, aiErr := service.CheckContent(req.Content)
	if aiErr != nil {
		logger.Log.Errorw("CreateWish: AI 审核失败", "error", aiErr)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": "内容审核失败，请稍后再试",
			"data":    gin.H{},
		})
		return
	}
	if isViolating {
		// 不安全，拒绝
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_FORBIDDEN_COMMENT,
			"message": "内容包含不当信息，发布被拒绝",
			"data":    gin.H{},
		})
		return
	}

	uidVal, _ := c.Get("userID")
	userID, _ := uidVal.(uint)

	// 组装 wish
	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	background := "default"
	if req.Background != nil && *req.Background != "" {
		background = *req.Background
	}

	w := model.Wish{
		UserID:     userID,
		Content:    req.Content,
		IsPublic:   isPublic,
		Background: background,
	}

	// 使用事务：创建 wish，创建 tags（如果有）
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&w).Error; err != nil {
			return err
		}
		if len(req.Tags) > 0 {
			var tags []model.WishTag
			for _, t := range req.Tags {
				if t == "" {
					continue
				}
				tags = append(tags, model.WishTag{
					WishID:  w.ID,
					TagName: t,
				})
			}
			if len(tags) > 0 {
				if err := tx.Create(&tags).Error; err != nil {
					return err
				}
			}
			}
			return nil
	}).
	if err != nil {
		logger.Log.Errorw("CreateWish: DB 创建失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": "发布愿望失败，请稍后重试",
			"data":    gin.H{},
		})
		return
	}

	// preload to include user/tags in response
	if err := db.Preload("User").Preload("Tags").First(&w, w.ID).Error; err != nil {
		logger.Log.Warnw("CreateWish: 创建成功但查询失败", "error", err, "wishID", w.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    apperr.SUCCESS,
		"message": "愿望发布成功",
		"data":    wishToResponse(w),
	})
}

// GetMyWishes GET /api/wishes/me
func GetMyWishes(c *gin.Context, db *gorm.DB) {
	uidVal, _ := c.Get("userID")
	userID, _ := uidVal.(uint)

	// 支持分页
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")
	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var wishes []model.Wish
	if err := db.Where("user_id = ?", userID).
		Preload("Tags").
		Order("created_at desc").
		Limit(pageSize).Offset(offset).
		Find(&wishes).Error; err != nil {
		logger.Log.Errorw("GetMyWishes: DB 查询失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": apperr.GetMsg(apperr.ERROR_SERVER_ERROR),
			"data":    gin.H{},
		})
		return
	}

	var resp []WishResponse
	for _, w := range wishes {
		resp = append(resp, wishToResponse(w))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    apperr.SUCCESS,
		"message": "获取我的愿望成功",
		"data": gin.H{
			"wishes": resp,
			"page":   page,
		},
	})
}

// DeleteWish DELETE /api/wishes/:id
func DeleteWish(c *gin.Context, db *gorm.DB) {
	idStr := c.Param("id")
	wishID64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_PARAM_INVALID,
			"message": "无效愿望 ID",
			"data":    gin.H{},
		})
		return
	}
	wishID := uint(wishID64)

	uidVal, _ := c.Get("userID")
	userID, _ := uidVal.(uint)

	// 查找 wish
	var wish model.Wish
	if err := db.First(&wish, wishID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"code":    apperr.ERROR_WISH_NOT_FOUND,
				"message": apperr.GetMsg(apperr.ERROR_WISH_NOT_FOUND),
				"data":    gin.H{},
			})
			return
		}
		logger.Log.Errorw("DeleteWish: 查询wish失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": apperr.GetMsg(apperr.ERROR_SERVER_ERROR),
			"data":    gin.H{},
		})
		return
	}

	// 检查权限：只能删除自己的愿望（后续可扩展管理员权限）
	if wish.UserID != userID {
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_UNAUTHORIZED,
			"message": "无权限删除该愿望",
			"data":    gin.H{},
		})
		return
	}

	// 软删除（GORM 的 Delete 会设置 DeletedAt）
	if err := db.Delete(&wish).Error; err != nil {
		logger.Log.Errorw("DeleteWish: 删除失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": "删除愿望失败，请稍后重试",
			"data":    gin.H{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    apperr.SUCCESS,
		"message": "删除愿望成功",
		"data":    gin.H{},
		})
}

// LikeWish POST /api/wishes/:id/like  -- 切换点赞
func LikeWish(c *gin.Context, db *gorm.DB) {
	idStr := c.Param("id")
	wishID64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_PARAM_INVALID,
			"message": "无效愿望 ID",
			"data":    gin.H{},
		})
		return
	}
	wishID := uint(wishID64)
	uidVal, _ := c.Get("userID")
	userID, _ := uidVal.(uint)

	// 检查 wish 是否存在
	var wish model.Wish
	if err := db.First(&wish, wishID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"code":    apperr.ERROR_WISH_NOT_FOUND,
				"message": apperr.GetMsg(apperr.ERROR_WISH_NOT_FOUND),
				"data":    gin.H{},
			})
			return
		}
		logger.Log.Errorw("LikeWish: 查询 wish 失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": apperr.GetMsg(apperr.ERROR_SERVER_ERROR),
			"data":    gin.H{},
		})
		return
	}

	var existing model.Like
	if err := db.Where("wish_id = ? AND user_id = ?", wishID, userID).First(&existing).Error; err == nil {
		// 已点赞 -> 取消点赞
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Wish{}).Where("id = ?", wishID).UpdateColumn("like_count", gorm.Expr("like_count - ?", 1)).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			logger.Log.Errorw("LikeWish: 取消点赞失败", "error", err)
			c.JSON(http.StatusOK, gin.H{
				"code":    apperr.ERROR_LIKE_FAILED,
				"message": apperr.GetMsg(apperr.ERROR_LIKE_FAILED),
				"data":    gin.H{},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.SUCCESS,
			"message": "已取消点赞",
			"data":    LikeToggleResponse{Liked: false},
			})
		return
	} else if err != gorm.ErrRecordNotFound {
		// 查询出错
		logger.Log.Errorw("LikeWish: 查询点赞失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_SERVER_ERROR,
			"message": apperr.GetMsg(apperr.ERROR_SERVER_ERROR),
			"data":    gin.H{},
		})
		return
	}

	// 未点赞 -> 创建点赞
	newLike := model.Like{
		WishID: wishID,
		UserID: userID,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newLike).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Wish{}).Where("id = ?", wishID).UpdateColumn("like_count", gorm.Expr("like_count + ?", 1)).Error; err != nil {
			return err
		}
		return nil
	}).Error
	if err != nil {
		logger.Log.Errorw("LikeWish: 创建点赞失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    apperr.ERROR_LIKE_FAILED,
			"message": apperr.GetMsg(apperr.ERROR_LIKE_FAILED),
			"data":    gin.H{},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apperr.SUCCESS,
		"message": "点赞成功",
		"data": LikeToggleResponse{Liked: true},
	})
}

// CreateComment POST /api/wishes/:id/comment
// ... rest of the code omitted for brevity ...