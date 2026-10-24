package page

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	contractsvc "github.com/cuihairu/croupier/internal/service"
)

var publishTestRevisionOne = 1

func modelProposalForPage(service *Service, ctx context.Context, pageKey string) (*model.PageProposal, error) {
	return model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
}

func TestPublishEntries_InvalidPageRejectedConsistently(t *testing.T) {
	for _, name := range []string{"accept-and-publish", "auto-repair"} {
		t.Run(name, func(t *testing.T) {
			service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
			proposal, err := freshInvalidProposal(t, service, ctx)
			require.NoError(t, err)
			var publishErr error
			switch name {
			case "publish":
				require.NoError(t, service.svcCtx.PageSpecModel.Delete(
					ctx, "demo-game", "development", proposal.PageKey))
				_, publishErr = service.Publish(ctx, &PagePublishRequest{
					PageKey: proposal.PageKey, DraftRevision: &publishTestRevisionOne,
				})
			case "republish":
				require.NoError(t, service.svcCtx.PageSpecModel.Delete(
					ctx, "demo-game", "development", proposal.PageKey))
				_, publishErr = service.RepublishDraftFromEntry(ctx, pagePublishEntry{
					Caller:          pagePublishCallerRepublish,
					RequiresPublish: true,
				}, &RepublishDraftRequest{PageKey: proposal.PageKey})
			case "accept-and-publish":
				proposal, proposalErr := freshInvalidProposal(t, service, ctx)
				if proposalErr != nil {
					publishErr = proposalErr
				} else {
					_, publishErr = proposalSvcFor(service, ctx).AcceptAndPublishProposal(
						ctx, "demo-game", "development", proposal.ProposalKey)
				}
			case "auto-repair":
				proposal, proposalErr := freshInvalidProposal(t, service, ctx)
				if proposalErr != nil {
					publishErr = proposalErr
				} else {
					publishErr = service.AutoPublishComposite(
						ctx, "demo-game", "development", proposal.ProposalKey)
				}
			}

			expectedError := "resource list query must map collection output"

			require.Error(t, publishErr)
			assert.Contains(t, publishErr.Error(), expectedError)
		})
	}
}

func TestAutoPublishComposite_InternalCallerOnly(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)
	proposal, err := modelProposalForPage(service, ctx, "resource--player")
	require.NoError(t, err)

	require.ErrorContains(t, service.AutoPublishComposite(ctx, "demo-game", "development", proposal.ProposalKey), "无权发布页面")
	published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.Error(t, err)
	assert.Nil(t, published)
}

func TestPublishEntries_StaleDraftRevisionRejectedConsistently(t *testing.T) {
	for _, name := range []string{"publish", "republish"} {
		t.Run(name, func(t *testing.T) {
			service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
			proposal, err := invalidDraftAfterPublication(t, service, ctx)
			require.NoError(t, err)
			staleRevision := proposal.DraftRevision - 1

			var publishErr error
			switch name {
			case "publish":
				_, publishErr = service.Publish(ctx, &PagePublishRequest{
					PageKey: proposal.PageKey, DraftRevision: &staleRevision,
				})
			case "republish":
				_, publishErr = service.RepublishDraftFromEntry(ctx, pagePublishEntry{
					Caller: pagePublishCallerRepublish, RequiresPublish: true,
				}, &RepublishDraftRequest{PageKey: proposal.PageKey, ExpectedDraftRevision: &staleRevision})
			}

			require.Error(t, publishErr)
			assert.Contains(t, publishErr.Error(), "草稿版本冲突")
		})
	}
}

func TestPublishEntries_PublishSnapshotConsistent(t *testing.T) {
	for _, name := range []string{"publish", "republish", "accept-and-publish", "auto-repair"} {
		t.Run(name, func(t *testing.T) {
			service, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish")
			seedResourceProposal(t, service)

			var response *PagePublishResponse
			var publishErr error
			switch name {
			case "publish", "republish":
				if name == "publish" {
					saved, saveErr := service.SaveDraft(ctx, validManualResourceDraft(t, service, 0))
					require.NoError(t, saveErr)
					response, publishErr = service.Publish(ctx, &PagePublishRequest{
						PageKey: "manual--resource", DraftRevision: &saved.DraftRevision,
					})
				} else {
					pageKey := "manual--resource"
					draftModel := model.NewPageSpecModel(service.svcCtx.DB)
					require.NoError(t, draftModel.Delete(ctx, "demo-game", "development", pageKey))
					_, saveErr := service.SaveDraft(ctx, validManualResourceDraft(t, service, 0))
					require.NoError(t, saveErr)
					response, publishErr = service.RepublishDraftFromEntry(ctx, pagePublishEntry{
						Caller: pagePublishCallerRepublish, RequiresPublish: true,
					}, &RepublishDraftRequest{PageKey: "manual--resource"})
				}
			case "accept-and-publish":
				proposal, proposalErr := modelProposalForPage(service, ctx, "resource--player")
				require.NoError(t, proposalErr)
				_, publishErr = proposalSvcFor(service, ctx).AcceptAndPublishProposal(
					ctx, "demo-game", "development", proposal.ProposalKey)
				published, latestErr := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(
					ctx, "demo-game", "development", "resource--player")
				require.NoError(t, latestErr)
				response = &PagePublishResponse{PageKey: published.PageKey, Published: true, PublishedVersion: published.Version}
			case "auto-repair":
				proposal, proposalErr := modelProposalForPage(service, ctx, "resource--player")
				require.NoError(t, proposalErr)
				publishErr = service.AutoPublishComposite(ctx, "demo-game", "development", proposal.ProposalKey)
				published, latestErr := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(
					ctx, "demo-game", "development", "resource--player")
				require.NoError(t, latestErr)
				response = &PagePublishResponse{PageKey: published.PageKey, Published: true, PublishedVersion: published.Version}
			}
			require.NoError(t, publishErr)
			require.NotNil(t, response)

			published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(
				ctx, "demo-game", "development", response.PageKey)
			require.NoError(t, err)
			require.True(t, published.Active)
			assert.Equal(t, response.PublishedVersion, published.Version)

			assert.Equal(t, rendererSchemaVersion, published.RendererSchemaVersion)
			records, _, listErr := auditStore.List(audit.AuditFilter{
				EventType: []audit.AuditEventType{audit.EventPagePublish},
			}, audit.AuditPage{PageSize: 100})
			require.NoError(t, listErr)
			coreRecords := make([]*audit.AuditRecord, 0, len(records))
			for _, record := range records {
				if record.Details["action"] == nil {
					coreRecords = append(coreRecords, record)
				}
			}
			require.Len(t, coreRecords, 1)
			assert.Equal(t, "success", coreRecords[0].Outcome)
			assert.Equal(t, response.PublishedVersion, coreRecords[0].Details["published_version"])
		})
	}
}

func proposalSvcFor(service *Service, ctx context.Context) *contractsvc.ProposalService {
	return service.newInternalProposalService()
}

func validManualResourceDraft(t *testing.T, service *Service, revision int) *PageSaveRequest {
	t.Helper()
	request := &PageSaveRequest{
		PageKey:       "manual--resource",
		DraftRevision: &revision,
		Type:          spec.PageTypeResource,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家资源", "en-US": "Player Resource"},
		Category:      spec.PageCategorySpec{Key: "player"},
		Resource: &spec.ResourcePageSpec{ListView: &spec.ListViewSpec{
			IdentityKey: "id",
			Columns:     []spec.ColumnSpec{{Key: "id"}},
			RowSchema:   spec.JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`),
		}},
		Bindings: proposalBindings(t, service),
	}
	return request
}

func proposalBindings(t *testing.T, service *Service) []spec.PageFunctionBinding {
	t.Helper()
	proposal, err := modelProposalForPage(service, context.Background(), "resource--player")
	require.NoError(t, err)
	pageSpec, err := pageSpecFromProposalModel(proposal)
	require.NoError(t, err)
	return pageSpec.Bindings
}

func invalidDraftAfterPublication(t *testing.T, service *Service, ctx context.Context) (*model.PageSpec, error) {
	seedResourceProposal(t, service)
	proposal, err := modelProposalForPage(service, ctx, "resource--player")
	if err != nil {
		return nil, err
	}
	if _, err := proposalSvcFor(service, ctx).AcceptAndPublishProposal(
		ctx, "demo-game", "development", proposal.ProposalKey); err != nil {
		return nil, err
	}

	draft, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(
		ctx, "demo-game", "development", proposal.PageKey)
	if err != nil {
		return nil, err
	}
	pageSpec, err := pageSpecFromModel(draft)
	if err != nil {
		return nil, err
	}
	pageSpec.Bindings[0].Selectors = &spec.BindingSelectors{}
	specJSON, err := pageJSONMarshal(pageSpec)
	if err != nil {
		return nil, err
	}
	draft.SpecJSON = string(specJSON)
	draft.DraftRevision++
	if err := service.svcCtx.PageSpecModel.Upsert(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func freshInvalidProposal(t *testing.T, service *Service, ctx context.Context) (*model.PageProposal, error) {
	db := service.svcCtx.DB
	rebuildFunctionContract(t, db, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.1",
		Risk:         "safe",
		Permission:   "player:query",
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"createdAt":{"type":"string"}}}},"total":{"type":"number"}}}`,
	})
	contractSvc := contractsvc.NewContractService(db)
	require.NoError(t, contractSvc.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, contractSvc.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))
	regenerated, err := modelProposalForPage(service, ctx, "resource--player")
	if err != nil {
		return nil, err
	}
	pageSpec, err := pageSpecFromProposalModel(regenerated)
	if err != nil {
		return nil, err
	}
	pageSpec.Bindings[0].Selectors = &spec.BindingSelectors{}
	specJSON, err := pageJSONMarshal(pageSpec)
	if err != nil {
		return nil, err
	}
	regenerated.PageSpec = model.JSON(specJSON)
	regenerated.Status = dbenum.ProposalStatusPending
	if err := model.NewPageProposalModel(db).UpsertProposal(ctx, regenerated); err != nil {
		return nil, err
	}
	return regenerated, nil
}
