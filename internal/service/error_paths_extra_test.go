package service

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
)

// Every call below runs against a closed database pool so the wrapped error
// returns of the services are exercised.

func TestErrorPaths_ProposalService(t *testing.T) {
	db := newClosedServiceDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	_, err := svc.ListProposals(ctx, "demo-game", "development")
	assert.Error(t, err)

	_, err = svc.ListProposalDTOs(ctx, "demo-game", "development", ProposalListFilter{})
	assert.Error(t, err)

	_, err = svc.GetProposalDTO(ctx, "demo-game", "development", "key")
	assert.Error(t, err)

	err = svc.AcceptProposal(ctx, "demo-game", "development", "key")
	assert.Error(t, err)

	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key")
	assert.Error(t, err)

	err = svc.RejectProposal(ctx, "demo-game", "development", "key")
	assert.Error(t, err)

	_, err = svc.Inbox(ctx, "demo-game", "development", ProposalListFilter{Status: "pending"})
	assert.Error(t, err)

	_, err = svc.listBlockedIssueDTOs(ctx, "g", "e", "player")
	assert.Error(t, err)

	_, err = svc.publishedContractChanges(ctx, "g", "e", "", nil)
	assert.Error(t, err)

	_, err = svc.draftContractChanges(ctx, "g", "e", "", nil, nil)
	assert.Error(t, err)

	_, err = svc.functionSpecsByID(ctx, "g", "e")
	assert.Error(t, err)

	// Missing scope details fail before any database access.
	err = svc.validateDirectPublishPageSpec(ctx, "", "", &model.PageProposal{}, testProposalPageSpec("op--x"))
	assert.Error(t, err)

	_, err = svc.buildBindingContracts(ctx, "g", "e", []spec.PageFunctionBinding{{ID: "b"}})
	assert.Error(t, err)

	persisted := &model.PageProposal{}
	persisted.ID = 1
	_, err = createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), persisted, "reason", "actor")
	assert.Error(t, err)
}

func TestErrorPaths_ContractService(t *testing.T) {
	db := newClosedServiceDB(t)
	ctx := proposalTestContext()
	svc := NewContractService(db)

	input := FunctionMetaInput{ID: "fn.x"}
	assert.Error(t, svc.RebuildContractFromFunctionMeta(ctx, "g", "e", "sdk", input))

	assert.Error(t, svc.RebuildResourceCapability(ctx, "g", "e", "player"))
	assert.Error(t, svc.removeResourceDerivedState(ctx, "g", "e", "player"))

	assert.Error(t, svc.RebuildProposalsForResource(ctx, "g", "e", "player"))
	assert.Error(t, svc.removeResourceProposal(ctx, "g", "e", "player"))
	assert.Error(t, svc.RebuildAllProposals(ctx, "g", "e"))

	assert.Error(t, svc.RebuildProposalForFunction(ctx, "g", "e", "fn.x"))

	_, err := svc.RemoveFunctionContract(ctx, "g", "e", "fn.x")
	assert.Error(t, err)

	_, err = svc.ListContracts(ctx, "g", "e")
	assert.Error(t, err)

	_, err = svc.GetContract(ctx, "g", "e", "fn.x")
	assert.Error(t, err)

	_, err = svc.ListResourceCapabilities(ctx, "g", "e")
	assert.Error(t, err)

	assert.Nil(t, svc.loadTermDictionary(ctx))
}
