package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProvisionConfig_EffectiveApproval(t *testing.T) {
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{Approval: "auto"}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalWorkflow, ProvisionConfig{Approval: "workflow"}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{Approval: "bogus"}.EffectiveApproval())
}

func TestBackupScheduleConfig_EffectiveInterval(t *testing.T) {
	t.Run("empty uses default", func(t *testing.T) {
		var c BackupScheduleConfig
		if got := c.EffectiveInterval(); got != DefaultBackupScheduleInterval {
			t.Fatalf("empty interval = %v, want default %v", got, DefaultBackupScheduleInterval)
		}
	})
	t.Run("valid parses", func(t *testing.T) {
		c := BackupScheduleConfig{Interval: "2h"}
		if got := c.EffectiveInterval(); got != 2*time.Hour {
			t.Fatalf("2h interval = %v, want 2h", got)
		}
	})
	t.Run("invalid falls back", func(t *testing.T) {
		for _, v := range []string{"not-a-duration", "-1h", "0s"} {
			c := BackupScheduleConfig{Interval: v}
			if got := c.EffectiveInterval(); got != DefaultBackupScheduleInterval {
				t.Fatalf("interval %q = %v, want default %v", v, got, DefaultBackupScheduleInterval)
			}
		}
	})
}
