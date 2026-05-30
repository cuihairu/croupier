export const EXTENSION_ERROR_CODES = {
  EXTENSION_ALREADY_INSTALLED: 'extension_already_installed',
  DEPENDENCY_BLOCKED: 'dependency_blocked',
  MISSING_DEPENDENCY: 'missing_dependency',
  VERSION_MISMATCH: 'version_mismatch',
  DEPENDENCY_CYCLE: 'dependency_cycle',
  FORBIDDEN: 'forbidden',
  NOT_FOUND: 'not_found',
} as const;

export type ExtensionErrorCode = (typeof EXTENSION_ERROR_CODES)[keyof typeof EXTENSION_ERROR_CODES];
