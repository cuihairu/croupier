/** Public surface of the Croupier Node.js plugin SDK. */

export {
  CORE_PROTOCOL_VERSION,
  CORE_PROTOCOL_VERSIONS,
  STOP_GRACE_SECONDS,
  PluginHandshakeError,
  bindTcp,
  controllerHandlers,
  handshakeLine,
  healthHandlers,
  negotiateProtocolVersion,
  requireCookie,
  resolveBind,
  servePlugin,
  stdioHandlers,
} from "./serve";
export type { ServeOptions } from "./serve";
export {
  BIND_ENV,
  COOKIE_KEY,
  COOKIE_VALUE,
  MULTIPLEX_KEY,
  PROTOCOL_VERSION,
  PROTOCOL_VERSIONS_KEY,
  failResult,
  makeProbeCall,
  normalizeDeadline,
  nowMs,
  outcomeToProto,
  passResult,
  probeHandlers,
  serve,
} from "./probeplugin";
export type {
  CallSurface,
  CheckOutcome,
  FailOptions,
  PassOptions,
  ProbeCall,
  ProbePlugin,
  ServeProbeOptions,
} from "./probeplugin";
export type {
  CheckRequestShape,
  CheckResultShape,
  DescribeResponseShape,
  ProtoModules,
} from "./proto";
export { loadProtos, resolveProtosDir } from "./proto";
