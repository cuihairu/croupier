package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvisionConfig_EffectiveApproval(t *testing.T) {
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{Approval: "auto"}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalWorkflow, ProvisionConfig{Approval: "workflow"}.EffectiveApproval())
	assert.Equal(t, ProvisionApprovalAuto, ProvisionConfig{Approval: "bogus"}.EffectiveApproval())
}
