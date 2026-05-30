export * from './auth';
export * from './functions';
export * from './games';
export * from './assignments';
export * from './registry';
export * from './ops';
export * from './audit';
export * from './storage';
export * from './me';
export * from './users';
export * from './roles';
export * from './messages';
export * from './entities';
export * from './support';
export * from './permissions';
export * from './players';
export * from './alerts';
export * from './nodes';
export * from './approvals';
export * from './admin';
export * from './function-calls';
export * from './terms';
export * from './extensions';
// Selective re-exports from enhanced APIs to avoid name conflicts.
export { getFunctionInstances, getRegistryServices } from './functions-enhanced';
export type { FunctionInstance, RegistryService } from './functions-enhanced';
// Types are colocated with each API module (functions/games/audit).
// No separate shared types barrel to avoid duplicate exports.
