package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Public Handlers
// ============================================================================

func GetSystemStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"configured": false})
}

func GetPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func GetRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rules": []interface{}{}})
}

func GetRuleDownload(c *gin.Context) {
	c.String(http.StatusOK, "# rule download stub")
}

// ============================================================================
// Auth Handlers
// ============================================================================

func AuthLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "auth login stub"})
}

func AuthCallback(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "auth callback stub"})
}

func AuthMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "auth me stub"})
}

// ============================================================================
// User Handlers
// ============================================================================

func UserPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func UserUpdateTime(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"update_time": ""})
}

func UserRefreshToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Download Handlers
// ============================================================================

func SubDownload(c *gin.Context) {
	c.String(http.StatusOK, "# subscription download stub")
}

func SubDownloadPreview(c *gin.Context) {
	c.String(http.StatusOK, "# subscription preview stub")
}

func SubDownloadToken(c *gin.Context) {
	c.String(http.StatusOK, "# subscription token download stub")
}

func ShareDownload(c *gin.Context) {
	c.String(http.StatusOK, "# share download stub")
}

// ============================================================================
// Admin: Setup
// ============================================================================

func PostConfigure(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func PostSwitchProvider(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func PostTestOIDC(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetOIDCConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"config": gin.H{}})
}

// ============================================================================
// Admin: Users
// ============================================================================

func ListUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []interface{}{}})
}

func GetUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": gin.H{}})
}

func UpdateUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeUserTokens(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadCustomSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadCustomSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteCustomSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func SwitchCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshCustomSubToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Subscriptions
// ============================================================================

func ListSubscriptions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"subscriptions": []interface{}{}})
}

func CreateSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"subscription": gin.H{}})
}

func UpdateSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Share Subscriptions
// ============================================================================

func ListShares(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"shares": []interface{}{}})
}

func CreateShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"share": gin.H{}})
}

func UpdateShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshShareToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeShareToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Platforms
// ============================================================================

func ListPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func CreatePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetPlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platform": gin.H{}})
}

func UpdatePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeletePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Rules
// ============================================================================

func ListAdminRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rules": []interface{}{}})
}

func CreateRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rule": gin.H{}})
}

func UpdateAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshRuleToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: System Config
// ============================================================================

func GetRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rate_limit_login": 10, "rate_limit_download": 20})
}

func UpdateRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Logs
// ============================================================================

func GetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
}
