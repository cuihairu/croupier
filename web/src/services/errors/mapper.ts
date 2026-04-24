import { EXTENSION_ERROR_CODES, type ExtensionErrorCode } from './codes';

export interface UiError {
  code: ExtensionErrorCode | 'unknown';
  title: string;
  message: string;
  details?: Record<string, any>;
}

function readBackendPayload(err: unknown): {
  code?: string;
  details?: Record<string, any>;
  message?: string;
} {
  const anyErr = err as {
    response?: { data?: { details?: Record<string, any>; message?: string } };
  };
  const details = anyErr?.response?.data?.details;
  return {
    code: typeof details?.code === 'string' ? details.code : undefined,
    details: details && typeof details === 'object' ? details : undefined,
    message: anyErr?.response?.data?.message,
  };
}

function normalizeCode(rawCode?: string): ExtensionErrorCode | 'unknown' {
  if (!rawCode) return 'unknown';
  switch (rawCode) {
    case 'dependency_missing':
      return EXTENSION_ERROR_CODES.MISSING_DEPENDENCY;
    case 'dependency_version_mismatch':
      return EXTENSION_ERROR_CODES.VERSION_MISMATCH;
    case EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED:
    case EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED:
    case EXTENSION_ERROR_CODES.MISSING_DEPENDENCY:
    case EXTENSION_ERROR_CODES.VERSION_MISMATCH:
    case EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE:
    case EXTENSION_ERROR_CODES.FORBIDDEN:
    case EXTENSION_ERROR_CODES.NOT_FOUND:
      return rawCode;
    default:
      return 'unknown';
  }
}

export function mapExtensionError(err: unknown): UiError {
  const payload = readBackendPayload(err);
  const code = normalizeCode(payload.code);
  switch (code) {
    case EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED:
      return {
        code,
        title: 'Extension Already Installed',
        message: 'This extension is already installed for the current scope.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED:
      return {
        code,
        title: 'Dependency Blocked',
        message: 'Uninstall blocked because other extensions still depend on it.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.MISSING_DEPENDENCY:
      return {
        code,
        title: 'Missing Dependency',
        message: 'Install blocked: required dependency is missing.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.VERSION_MISMATCH:
      return {
        code,
        title: 'Version Mismatch',
        message: 'Version constraint not satisfied.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE:
      return {
        code,
        title: 'Dependency Cycle',
        message: 'Dependency graph contains a cycle.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.FORBIDDEN:
      return {
        code,
        title: 'Forbidden',
        message: 'You do not have permission for this operation.',
        details: payload.details,
      };
    case EXTENSION_ERROR_CODES.NOT_FOUND:
      return {
        code,
        title: 'Not Found',
        message: 'Requested resource was not found.',
        details: payload.details,
      };
    default:
      return {
        code: 'unknown',
        title: 'Unexpected Error',
        message: payload.message || 'Please retry or contact an administrator.',
        details: payload.details,
      };
  }
}
