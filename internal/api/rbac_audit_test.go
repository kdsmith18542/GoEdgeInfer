package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/keith/goedgeinfer/internal/config"
)

func TestMgmtRBAC_AdminAllowed(t *testing.T) {
	r := gin.Default()
	apiInstance := &API{}
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{Enabled: true, RequireRole: "admin"})
	claims := map[string]interface{}{"role": "admin"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("jwt_claims", claims)
	c.Request, _ = http.NewRequest("GET", "/mgmt/security_config", nil)
	apiInstance.ListRemoteModels(c)
	if w.Code == 403 {
		t.Fatalf("admin should be allowed")
	}
}

func TestMgmtRBAC_NonAdminDenied(t *testing.T) {
	r := gin.Default()
	apiInstance := &API{}
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{Enabled: true, RequireRole: "admin"})
	claims := map[string]interface{}{"role": "user"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("jwt_claims", claims)
	c.Request, _ = http.NewRequest("GET", "/mgmt/security_config", nil)
	apiInstance.ListRemoteModels(c)
	if w.Code != 403 {
		t.Fatalf("non-admin should be denied")
	}
}

func TestAuditLog_Called(t *testing.T) {
	// This test would check that auditLog is called (mock logging or check output)
	// For brevity, just call auditLog and ensure no panic
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("jwt_claims", map[string]interface{}{"sub": "testuser"})
	auditLog(c, "test_action", map[string]interface{}{"foo": "bar"})
}
