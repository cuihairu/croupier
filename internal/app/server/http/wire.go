//go:build wireinject

package httpserver

import (
    dom "github.com/cuihairu/croupier/internal/ports"
    repog "github.com/cuihairu/croupier/internal/repo/gorm/games"
    svcg "github.com/cuihairu/croupier/internal/service/games"
    "github.com/google/wire"
)

// Provider sets for Games domain (Ports/Adapters + Service).
var GamesRepoSet = wire.NewSet(
    repog.NewRepo,       // GORM repo
    repog.NewPortRepo,   // adapter to ports
    wire.Bind(new(dom.GamesRepository), new(*repog.PortRepo)),
)

var GamesServiceSet = wire.NewSet(
    svcg.NewService,
)

// InitServerApp composes *Server with DB, repo, service and external deps.
func InitServerApp(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy rbac.PolicyInterface, reg *registry.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }) (*Server, error) {
    wire.Build(
        ProvideGormDBFromEnv,
        repog.AutoMigrate,
        GamesRepoSet,
        ProvideGamesDefaults,
        GamesServiceSet,
        ProvideCertStore,
        ProvideObjectStoreFromEnv,
        ProvideClickHouseFromEnv,
        initServerWithDeps,
    )
    return nil, nil
}

// InitServerAppAuto composes *Server by constructing audit/rbac/jwt from environment.
func InitServerAppAuto(descriptorDir string, invoker FunctionInvoker, reg *registry.Store, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }) (*Server, error) {
    wire.Build(
        ProvideGormDBFromEnv,
        repog.AutoMigrate,
        GamesRepoSet,
        ProvideGamesDefaults,
        GamesServiceSet,
        ProvideAuditWriterDefault,
        ProvideRBACPolicyAuto,
        ProvideJWTManagerFromEnv,
        ProvideCertStore,
        ProvideObjectStoreFromEnv,
        ProvideClickHouseFromEnv,
        initServerAuto,
    )
    return nil, nil
}
