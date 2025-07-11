package metrics

import (
	"testing"
)

func TestRegisterSysMetrics_Idempotent(t *testing.T) {
	RegisterSysMetrics()
	RegisterSysMetrics() // should not panic
}

func TestUpdateSysMetrics(t *testing.T) {
	updateSysMetrics() // should not panic
}
