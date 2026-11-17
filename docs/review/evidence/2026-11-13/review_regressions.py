"""Ordinary behavior regression cases; assertions state the desired behavior.
Uses temporary Go overlays, so no business source or existing tests are edited.
The current implementation is expected to fail these assertions; that is evidence,
not an indication that the findings have been fixed.
"""
import json
import pathlib
import subprocess
import tempfile
ROOT = pathlib.Path(__file__).resolve().parents[4]
HERE = pathlib.Path(__file__).resolve().parent
CASES = {
'internal/svc': r'''package svc
import (
 "context"
 "testing"
 "github.com/cuihairu/croupier/internal/ops/provision"
 "github.com/stretchr/testify/require"
)
func TestReviewProvisionAdapterCanUpdateExistingRun(t *testing.T) {
 store:=newProvisionDBStore(newProvisionWiringModel(t))
 run:=wiringRunFixture()
 require.NoError(t,store.Create(context.Background(),run))
 run.State=provision.StateApplying
 require.NoError(t,store.Update(context.Background(),run),"updating an existing run must update its row rather than insert ID=0")
 saved,err:=store.Get(context.Background(),run.ID);require.NoError(t,err)
 require.Equal(t,provision.StateApplying,saved.State)
}
''',
'internal/api/incident': r'''package incident
import (
 "context"
 "testing"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/stretchr/testify/require"
)
func TestReviewIncidentDetailsUpdateRoundtrip(t *testing.T) {
 service:=setupService(t);ctx:=context.Background()
 category:=mustCategory(t,service,model.IncidentCatArt)
 row,_,err:=service.CreateIncident(ctx,&IncidentCreateRequest{Title:"normal incident",CategoryID:category.ID,Details:map[string]interface{}{"summary":"old"}},"reviewer")
 require.NoError(t,err)
 updated,err:=service.UpdateIncident(ctx,row.ID,&IncidentUpdateRequest{Details:map[string]interface{}{"summary":"new"}})
 require.NoError(t,err,"ordinary JSON details update must serialize the map before binding SQL parameters")
 require.Equal(t,"new",updated.Details["summary"])
}
''',
'core/healthprobe': r'''package healthprobe
import (
 "context"
 "net"
 "testing"
 "time"
 "golang.org/x/net/icmp"
 "golang.org/x/net/ipv6"
 "github.com/stretchr/testify/require"
)
func TestReviewIPv6ValidEchoReplySucceeds(t *testing.T) {
 conn:=&fakeICMPConn{reply:func(request []byte)([]byte,error){return echoReplyOf(t,icmpProtocolIPv6,request,nil,ipv6.ICMPTypeEchoReply),nil}}
 checker:=NewICMPChecker("::1",time.Second,1)
 checker.open=func(string,string)(net.PacketConn,error){return conn,nil}
 err:=checker.Check(context.Background())
 require.NoError(t,err,"a valid IPv6 echo reply with matching ID, sequence and payload must pass")
}
// Ensure the test checks the actual ICMP type interface used by the implementation.
var _ icmp.Type = ipv6.ICMPTypeEchoReply
''',
'internal/ops/deploy': r'''package deploy
import (
 "context"
 "testing"
 "time"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/stretchr/testify/require"
)
func TestReviewRollbackRejectsRunningDeployment(t *testing.T) {
 store:=newFakeStore();agent:=newFakeAgent();o:=New(store,agent)
 d,err:=NewDeployment("pkg-1","review","game","dev","game-server","game",100,[]string{"a"},"operator")
 require.NoError(t,err);d.State=model.DeploymentStateDeploying
 require.NoError(t,store.Create(context.Background(),d))
 // A second request arrives while the first request is still preparing the target.
 _,err=o.Rollback(context.Background(),d,pkg())
 require.Error(t,err,"rollback must reject or coordinate with active deployment, not report rolled_back without any operation")
 require.Empty(t,agent.applied)
}
type reviewCancelAtHealthy struct{*fakeAgent;cancel context.CancelFunc}
func(a *reviewCancelAtHealthy) Healthy(context.Context,string)(bool,string){a.cancel();return true,""}
func TestReviewCancelledBatchRetainsCompletedTarget(t *testing.T) {
 store:=newFakeStore();base:=newFakeAgent()
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 agent:=&reviewCancelAtHealthy{fakeAgent:base,cancel:cancel}
 o:=New(store,agent);o.BatchWait=time.Second
 d,err:=NewDeployment("pkg-1","review","game","dev","game-server","game",100,[]string{"a"},"operator")
 require.NoError(t,err);require.NoError(t,store.Create(context.Background(),d))
 _,err=o.Start(ctx,d,pkg());require.ErrorIs(t,err,context.Canceled)
 require.Len(t,base.applied,1,"the binary was already updated before cancellation")
 saved,err:=store.FindByID(context.Background(),d.DeploymentID);require.NoError(t,err)
 targets:=decodeTargets(saved.TargetsJSON)
 require.Equal(t,model.DeploymentTargetHealthy,targets[0].State,"completed target progress must be persisted before cancellable batch waiting")
}
'''
}
CASES['core/pluginhost'] = r'''package pluginhost
import (
 "context"
 "testing"
 "time"
)
func TestReviewPluginStartupDoesNotBlockAnotherCallerDeadline(t *testing.T) {
 controller:=newEchoController(t,Knobs{})
 original:=controller.ct.Describe
 entered:=make(chan struct{});release:=make(chan struct{})
 controller.ct.Describe=func(ctx context.Context,impl testImpl)(string,string,error){close(entered);<-release;return original(ctx,impl)}
 firstDone:=make(chan struct{})
 go func(){defer close(firstDone);controller.Call(context.Background(),"echo",time.Second,func(context.Context,testImpl)error{return nil})}()
 select {case <-entered:case <-time.After(10*time.Second):t.Fatal("test plugin did not reach Describe")}
 secondDone:=make(chan struct{})
 ctx,cancel:=context.WithTimeout(context.Background(),10*time.Millisecond);defer cancel()
 go func(){defer close(secondDone);controller.Call(ctx,"echo",time.Second,func(ctx context.Context,_ testImpl)error{return ctx.Err()})}()
 select {
 case <-secondDone:
 case <-time.After(100*time.Millisecond):t.Error("a caller with a 10ms deadline is still waiting behind another plugin startup after 100ms")
 }
 close(release)
 <-firstDone; <-secondDone
 controller.Stop()
}
'''
CASES['internal/svc'] = CASES['internal/svc'].replace('"github.com/cuihairu/croupier/internal/ops/provision"','"github.com/cuihairu/croupier/internal/ops/provision"\n "github.com/cuihairu/croupier/internal/platform/approvals"') + r'''
type reviewCASConflictStore struct {provision.RunStore}
func(s *reviewCASConflictStore) CompareAndSwapState(ctx context.Context,id,from,to string)(bool,*provision.Run,error){
 // Another caller wins after our Get and before our CAS.
 _,_,err:=s.RunStore.CompareAndSwapState(ctx,id,from,to)
 if err!=nil{return false,nil,err}
 return s.RunStore.CompareAndSwapState(ctx,id,from,to)
}
func TestReviewProvisionCASConflictReturnsErrorWithoutPanic(t *testing.T) {
 store:=newProvisionDBStore(newProvisionWiringModel(t));ctx:=context.Background()
 run:=wiringRunFixture();run.ApprovalID="review-approved"
 require.NoError(t,store.Create(ctx,run))
 approvalsStore:=approvals.NewMemStore()
 _,err:=approvalsStore.Create(&approvals.Approval{ID:run.ApprovalID,State:"pending",Actor:"creator"});require.NoError(t,err)
 _,err=approvalsStore.Approve(run.ApprovalID,"reviewer");require.NoError(t,err)
 orchestrator:=&provision.Orchestrator{Store:&reviewCASConflictStore{RunStore:store},Approval:newProvisionApprovalGate(approvalsStore)}
 require.NotPanics(t,func(){
  _,err:=orchestrator.ResumeAsync(ctx,run.ID)
  require.ErrorIs(t,err,provision.ErrRunNotPending)
 },"a normal competing resume must return conflict, not dereference the adapter's nil current row")
}
'''
CASES['internal/tasks/scheduler'] = r'''package scheduler
import (
 "context"
 "testing"
 "time"
 "github.com/stretchr/testify/require"
)
func TestReviewSameDueOccurrenceHasOneClaimAcrossMinuteBoundary(t *testing.T) {
 ctx:=context.Background()
 due:=time.Date(2026,11,13,10,30,0,0,time.UTC)
 store:=newFakeStore();dispatcher:=&fakeDispatcher{}
 one:=NewManager(store,dispatcher);two:=NewManager(store,dispatcher)
 // Both managers loaded the same due occurrence before either updated its next trigger.
 first:=dueSchedule(1,"30 10 * * *","",0,5,due)
 second:=first
 one.triggerSchedule(ctx,&first,due.Add(59*time.Second))
 two.triggerSchedule(ctx,&second,due.Add(time.Minute))
 require.Len(t,dispatcher.calls,1,"two loaded copies of the same once-daily occurrence must share the same claim key even across a minute boundary")
}
'''
with tempfile.TemporaryDirectory(prefix='croupier-code-review-') as temp:
    replacements={}
    for index,(package,source) in enumerate(CASES.items()):
        path=pathlib.Path(temp)/f'case_{index}_test.go'
        path.write_text(source)
        (HERE/f'case_{index}_test.go.txt').write_text(source)
        replacements[str(ROOT/package/'code_review_overlay_test.go')]=str(path)
    overlay=pathlib.Path(temp)/'overlay.json'
    overlay.write_text(json.dumps({'Replace':replacements}))
    cmd=['go','test','-overlay',str(overlay),'-count=1','-timeout','3m','-run','^TestReview','-v']+['./'+p for p in CASES]
    result=subprocess.run(cmd,cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
    (HERE/'regressions.log').write_text(result.stdout)
    print(result.stdout,end='')
    raise SystemExit(result.returncode)
