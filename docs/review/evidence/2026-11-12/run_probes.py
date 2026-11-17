#!/usr/bin/env python3
"""Read-only audit probes via Go overlays; PASS means the documented defect reproduced."""
import json
import pathlib
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[4]
HERE = pathlib.Path(__file__).resolve().parent
SOURCES = {
'internal/security/jwtutil': r'''package jwtutil
import (
 "testing"
 "github.com/cuihairu/croupier/internal/config"
)
func TestDeepAuditProductionSecret(t *testing.T) {
 t.Setenv("CROUPIER_MODE", "")
 t.Setenv("CROUPIER_ENV", "")
 for _, mode := range []string{"prod", "production", "release"} {
  cfg := config.Config{}
  cfg.Server.Mode = mode
  secret, err := ResolveSecret(cfg)
  if err != nil || secret != DevSecret() { t.Fatalf("behavior changed for %s: err=%v", mode, err) }
  t.Logf("REPRODUCED: server.mode=%s with no secret returns public development key", mode)
 }
}
''',
'internal/api/backup': r'''package backup
import (
 "context"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"
 "time"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/cuihairu/croupier/internal/security/jwtutil"
 "github.com/cuihairu/croupier/internal/svc"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)
func TestDeepAuditUnprivilegedBackupDownload(t *testing.T) {
 env := setupBackupFlowEnv(t)
 env.svcCtx.AdminModel = model.NewAdminModel(env.db)
 a := &model.Admin{Username:"audit_no_permissions", Status:1}
 require.NoError(t, env.db.Create(a).Error)
 var roles int64
 require.NoError(t, env.db.Model(&model.AdminRole{}).Where("admin_id = ?",a.ID).Count(&roles).Error)
 require.Zero(t, roles)
 secret := "local-audit-only-random-purpose-key"
 defer jwtutil.ResetGlobalSecretForTesting(secret)()
 token, err := jwtutil.Sign(secret,a.Username,nil,a.ID,0,time.Now())
 require.NoError(t, err)
 file := filepath.Join(t.TempDir(),"meta.sqlite")
 require.NoError(t,os.WriteFile(file,[]byte("AUDIT_PRIVATE_DATABASE_CONTENT"),0600))
 createBackupRow(t,env,&model.Backup{BackupID:"audit-sensitive",Name:"meta",Status:"succeeded",Location:file})
 r:=gin.New()
 r.Use(svc.NewAuthMiddleware(env.svcCtx))
 r.GET("/api/v1/backups",env.handler.List)
 r.GET("/api/v1/backups/:id/download",env.handler.Download)
 r.DELETE("/api/v1/backups/:id",env.handler.Delete)
 for _, probe := range []struct{method,path string}{
  {http.MethodGet,"/api/v1/backups"},
  {http.MethodGet,"/api/v1/backups/audit-sensitive/download"},
  {http.MethodDelete,"/api/v1/backups/audit-sensitive"},
 } {
  req:=httptest.NewRequest(probe.method,probe.path,nil)
  req.Header.Set("Authorization","Bearer "+token)
  w:=httptest.NewRecorder(); r.ServeHTTP(w,req)
  require.Equal(t,200,w.Code,w.Body.String())
  if probe.path=="/api/v1/backups/audit-sensitive/download" { require.Contains(t,w.Body.String(),"AUDIT_PRIVATE_DATABASE_CONTENT") }
  t.Logf("REPRODUCED: account with zero roles: %s %s -> %d",probe.method,probe.path,w.Code)
 }
 _,err=env.svcCtx.BackupModel.FindByBackupID(context.Background(),"audit-sensitive")
 require.Error(t,err)
}
''',
'internal/platform/backupexec': r'''package backupexec
import (
 "context"
 "os"
 "path/filepath"
 "testing"
 gsqlite "github.com/glebarez/sqlite"
 "github.com/stretchr/testify/require"
 "gorm.io/gorm"
)
func TestDeepAuditSQLiteWALLoss(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"live.db")
 db,err:=gorm.Open(gsqlite.Open(path),&gorm.Config{})
 require.NoError(t,err)
 sqlDB,err:=db.DB(); require.NoError(t,err); defer sqlDB.Close()
 require.NoError(t,db.Exec("CREATE TABLE audit_receipts (id INTEGER PRIMARY KEY)").Error)
 var mode string
 require.NoError(t,db.Raw("PRAGMA journal_mode=WAL").Scan(&mode).Error)
 require.Equal(t,"wal",mode)
 require.NoError(t,db.Exec("PRAGMA wal_autocheckpoint=0").Error)
 require.NoError(t,db.Exec("INSERT INTO audit_receipts(id) VALUES(42)").Error)
 var live int64
 require.NoError(t,db.Table("audit_receipts").Count(&live).Error); require.EqualValues(t,1,live)
 wal,err:=os.Stat(path+"-wal"); require.NoError(t,err); require.Positive(t,wal.Size())
 e:=New("sqlite",path,nil,nil,"")
 out,_,checksum,err:=e.dump(context.Background(),"audit-wal")
 require.NoError(t,err); defer os.Remove(out); require.Len(t,checksum,64)
 copyDB,err:=gorm.Open(gsqlite.Open(out),&gorm.Config{}); require.NoError(t,err)
 copySQL,err:=copyDB.DB(); require.NoError(t,err); defer copySQL.Close()
 var restored int64
 require.NoError(t,copyDB.Table("audit_receipts").Count(&restored).Error)
 require.Zero(t,restored)
 t.Logf("REPRODUCED: live committed rows=%d, successful hashed backup rows=%d",live,restored)
}
''',
'internal/svc': r'''package svc
import (
 "context"
 "testing"
 "time"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/cuihairu/croupier/internal/security/jwtutil"
 gsqlite "github.com/glebarez/sqlite"
 "github.com/stretchr/testify/require"
 "gorm.io/gorm"
)
func TestDeepAuditDeletedAccountStillAuthenticates(t *testing.T) {
 db,err:=gorm.Open(gsqlite.Open("file:audit_deleted?mode=memory&cache=shared"),&gorm.Config{})
 require.NoError(t,err); require.NoError(t,db.AutoMigrate(&model.Admin{}))
 conn,err:=db.DB(); require.NoError(t,err); defer conn.Close()
 a:=&model.Admin{Username:"audit_deleted",Status:1}
 require.NoError(t,db.Create(a).Error)
 secret:="audit-local-only-secret"
 defer jwtutil.ResetGlobalSecretForTesting(secret)()
 token,err:=jwtutil.Sign(secret,a.Username,nil,a.ID,0,time.Now()); require.NoError(t,err)
 require.NoError(t,db.Delete(a).Error)
 m:=NewAuthMiddlewareImpl(&ServiceContext{DB:db,AdminModel:model.NewAdminModel(db)})
 name,_,id,err:=m.authenticate(context.Background(),token)
 require.NoError(t,err); require.Equal(t,a.Username,name); require.Equal(t,a.ID,id)
 t.Log("REPRODUCED: soft-deleted account accepted by fresh middleware (no cache involved)")
}
''',
'internal/api/component': r'''package component
import (
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)
func TestDeepAuditTemplateWriteWithoutPermission(t *testing.T) {
 db:=setupV4DB(t)
 h:=NewHandler(model.NewComponentTemplateModel(db),model.NewFunctionContractModel(db))
 r:=gin.New()
 // Model the identity provided by the production Authority middleware; no permissions/roles.
 r.Use(func(c *gin.Context){c.Set("username","audit_viewer");c.Set("adminID",uint(123));c.Next()})
 h.Register(r.Group("/api/v1/component-templates"))
 req:=httptest.NewRequest("POST","/api/v1/component-templates",strings.NewReader(`{"key":"audit-global","name":{"en-US":"Audit"},"tree":[]}`))
 req.Header.Set("Content-Type","application/json")
 w:=httptest.NewRecorder();r.ServeHTTP(w,req)
 require.Equal(t,200,w.Code,w.Body.String())
 t.Log("REPRODUCED: template create succeeds with identity only, no role/permission/owner checks")
}
'''
}

SOURCES['internal/powerops'] = r'''package powerops
import (
 "context"
 "testing"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/stretchr/testify/require"
)
type auditBarrierGate struct {
 *fakeGate
 arrived chan struct{}
 release chan struct{}
}
func(g *auditBarrierGate) Approved(ctx context.Context,id string)(bool,error) {
 g.arrived<-struct{}{}
 <-g.release
 return g.fakeGate.Approved(ctx,id)
}
func TestDeepAuditPowerDoubleResume(t *testing.T) {
 h:=newHarness(t,[]string{"on"})
 req:=&model.PowerRequest{RequestID:"audit-double-resume",State:StatePending,ApprovalID:"audit-approved",Action:string(ActionReboot)}
 require.NoError(t,h.store.Create(req))
 h.gate.approve(req.ApprovalID)
 gate:=&auditBarrierGate{fakeGate:h.gate,arrived:make(chan struct{},2),release:make(chan struct{})}
 h.orch.Approval=gate
 results:=make(chan error,2)
 for i:=0;i<2;i++ {go func(){_,err:=h.orch.BeginResume(context.Background(),req.RequestID);results<-err}()}
 <-gate.arrived; <-gate.arrived
 close(gate.release)
 require.NoError(t,<-results);require.NoError(t,<-results)
 t.Log("REPRODUCED: both concurrent resume calls pass pending/approval checks and persist executing")
}
func TestDeepAuditPowerCountsDisagree(t *testing.T) {
 h:=newHarness(t,[]string{"off"})
 req:=&model.PowerRequest{RequestID:"audit-counts",State:StateVerifying,Total:1}
 require.NoError(t,h.store.Create(req))
 targets:=[]*model.PowerRequestTarget{{RequestID:req.RequestID,NodeID:"n1",Provider:ProviderIPMI,Success:true,Verified:false}}
 final:=terminalState(ActionReboot,targets)
 require.Equal(t,StateFailed,final)
 h.orch.finish(req,targets,final)
 row,err:=h.store.Get(req.RequestID);require.NoError(t,err)
 require.Equal(t,StateFailed,row.State);require.Equal(t,1,row.Succeeded);require.Equal(t,0,row.Failed)
 t.Logf("REPRODUCED: state=%s succeeded=%d failed=%d for one unverified machine",row.State,row.Succeeded,row.Failed)
}
'''
SOURCES['internal/tasks'] = r'''package tasks
import (
 "context"
 "testing"
 "time"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/stretchr/testify/require"
)
func TestDeepAuditDormantScopeReaper(t *testing.T) {
 store,_,dbs:=setupMultiGame(t)
 db:=dbs["alpha/prod"]
 require.NoError(t,db.Create(scopeRun("audit-orphan","old-agent",StatusRunning)).Error)
 require.NoError(t,db.Model(&model.TaskRun{}).Where("task_id = ?","audit-orphan").UpdateColumn("updated_at",time.Now().Add(-time.Hour)).Error)
 // A fresh production Router.OpenScopes is empty after process restart.
 resolver:=&fakeScopes{dbs:dbs}
 store.SetScopeResolver(resolver)
 n,err:=store.ReapStale(context.Background(),time.Minute,StatusTimedOut,"budget")
 require.NoError(t,err);require.Zero(t,n)
 var row model.TaskRun
 require.NoError(t,db.Where("task_id = ?","audit-orphan").First(&row).Error)
 require.Equal(t,StatusRunning,row.Status)
 resolver.scopes=[]TaskScope{{GameID:"alpha",Env:"prod"}}
 n,err=store.ReapStale(context.Background(),time.Minute,StatusTimedOut,"budget")
 require.NoError(t,err);require.EqualValues(t,1,n)
 t.Log("REPRODUCED: stale persisted task stays running until its scope enters current-process OpenScopes")
}
'''

SOURCES['internal/transport/tcp'] = r'''package tcp
import (
 "context"
 "encoding/json"
 "fmt"
 "testing"
 "time"
 transportcore "github.com/cuihairu/croupier/core/transport"
 sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
 "github.com/cuihairu/croupier/pkg/protocol"
 "github.com/stretchr/testify/require"
 "google.golang.org/protobuf/proto"
)
func TestDeepAuditHandlerErrorsLookSuccessful(t *testing.T) {
 h:=transportcore.HandlerFunc(func(context.Context,uint32,uint32,[]byte)([]byte,error){return nil,fmt.Errorf("bad \"quoted\"\nmessage")})
 srv,err:=NewServer(&Config{Address:"127.0.0.1:0",Insecure:true},h);require.NoError(t,err)
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 done:=make(chan struct{});go func(){defer close(done);_ = srv.Serve(ctx)}()
 defer func(){_ = srv.Close();<-done}()
 client,err:=NewClient(&Config{Address:srv.Addr(),Insecure:true});require.NoError(t,err);defer client.Close()
 _,body,err:=client.Call(ctx,protocol.MsgInvokeRequest,nil);require.NoError(t,err)
 resp:=&sdkv1.InvokeResponse{};require.NoError(t,proto.Unmarshal(body,resp))
 require.False(t,json.Valid(resp.Payload))
 t.Logf("REPRODUCED: Invoke handler error -> Call err=nil, malformed JSON payload=%q",resp.Payload)
 _,body,err=client.Call(ctx,protocol.MsgStartTaskRequest,nil);require.NoError(t,err)
 require.Empty(t,body)
 require.NoError(t,proto.Unmarshal(body,&sdkv1.StartTaskResponse{}))
 t.Log("REPRODUCED: failed StartTask handler -> successful Call, empty zero-value protobuf response")
}
'''
SOURCES['internal/api/storage'] = r'''package storage
import (
 "context"
 "io"
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)
func TestDeepAuditObjectOverwriteWithoutPermission(t *testing.T) {
 sc:=setupSvcCtxWithStore(t)
 require.NoError(t,sc.ObjectStore.Put(context.Background(),"backups/private.sqlite",strings.NewReader("original"),8,"application/octet-stream"))
 h:=NewHandler(NewService(sc));r:=gin.New()
 r.Use(func(c *gin.Context){c.Set("username","audit_no_permissions");c.Set("adminID",uint(1));c.Next()})
 r.POST("/api/v1/storage/objects",h.UploadObject)
 req:=httptest.NewRequest("POST","/api/v1/storage/objects",strings.NewReader(`{"path":"backups/private.sqlite","content":"replacement"}`))
 req.Header.Set("Content-Type","application/json")
 w:=httptest.NewRecorder();r.ServeHTTP(w,req);require.Equal(t,200,w.Code,w.Body.String())
 reader,err:=sc.ObjectStore.Get(context.Background(),"backups/private.sqlite");require.NoError(t,err);defer reader.Close()
 content,err:=io.ReadAll(reader);require.NoError(t,err);require.Equal(t,"replacement",string(content))
 t.Log("REPRODUCED: login identity with no permissions overwrites an object under backups/ via storage API")
}
'''
SOURCES['internal/api/ops'] = r'''package ops
import (
 "context"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
 "github.com/cuihairu/croupier/internal/model"
 "github.com/cuihairu/croupier/internal/security/jwtutil"
 "github.com/cuihairu/croupier/internal/svc"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)
func TestDeepAuditPowerConfigWithoutPermission(t *testing.T) {
 h:=newPowerHarness(t,true,[]string{"on"})
 require.NoError(t,h.db.AutoMigrate(&model.Admin{},&model.AdminRole{},&model.Role{},&model.Game{},&model.GameEnvBinding{},&model.AdminGameEnvScope{}))
 a:=&model.Admin{Username:"audit_game_viewer",Status:1};require.NoError(t,h.db.Create(a).Error)
 game:=&model.Game{Name:"audit-limited",GameID:"audit-limited",AliasName:"audit"};require.NoError(t,h.db.Create(game).Error)
 require.NoError(t,h.db.Create(&model.GameEnvBinding{GameID:game.GameID,Env:"dev",DatabaseName:"game_audit_dev"}).Error)
 require.NoError(t,h.db.Create(&model.AdminGameEnvScope{AdminID:a.ID,GameID:game.ID,Env:"dev"}).Error)
 sc:=h.service.svcCtx;sc.AdminModel=model.NewAdminModel(h.db);sc.GameModel=model.NewGameModel(h.db)
 secret:="local-audit-ops-test-key";defer jwtutil.ResetGlobalSecretForTesting(secret)()
 token,err:=jwtutil.Sign(secret,a.Username,nil,a.ID,0,time.Now());require.NoError(t,err)
 r:=gin.New();r.Use(svc.NewAuthMiddleware(sc));g:=r.Group("/api/v1/ops",svc.GameDBMiddleware(sc))
 g.PUT("/nodes/:nodeId/power-config",h.handler.PowerConfigPut)
 req:=httptest.NewRequest("PUT","/api/v1/ops/nodes/node-1/power-config",strings.NewReader(`{"provider":"ipmi","bmcAddr":"192.0.2.10"}`))
 req.Header.Set("Content-Type","application/json");req.Header.Set("Authorization","Bearer "+token)
 req.Header.Set("X-Game-ID",game.GameID);req.Header.Set("X-Env","dev")
 w:=httptest.NewRecorder();r.ServeHTTP(w,req);require.Equal(t,200,w.Code,w.Body.String())
 node,err:=sc.NodeModel.FindByNodeID(context.Background(),"node-1");require.NoError(t,err)
 require.Contains(t,node.Meta["power"],"bmcAddr")
 t.Log("REPRODUCED: zero-role account with valid game/dev scope changed global node power configuration through real Auth+GameDBMiddleware")
}
'''

def main():
    with tempfile.TemporaryDirectory(prefix='croupier-deep-audit-') as folder:
        temp = pathlib.Path(folder)
        replacements = {}
        for i, (package, source) in enumerate(SOURCES.items()):
            file = temp / f'probe_{i}_test.go'
            file.write_text(source)
            (HERE / f'probe_{i}_test.go.txt').write_text(source)
            replacements[str(ROOT / package / 'deep_audit_overlay_test.go')] = str(file)
        overlay = temp / 'overlay.json'
        overlay.write_text(json.dumps({'Replace': replacements}))
        cmd = ['go', 'test', '-overlay', str(overlay), '-count=1', '-run', '^TestDeepAudit', '-v', '-timeout', '180s'] + ['./'+p for p in SOURCES]
        result = subprocess.run(cmd, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        (HERE / 'go-probes.log').write_text(result.stdout)
        print(result.stdout, end='')
        return result.returncode

if __name__ == '__main__':
    raise SystemExit(main())
