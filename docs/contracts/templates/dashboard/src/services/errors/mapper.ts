import { EXTENSION_ERROR_CODES, type ExtensionErrorCode } from "./codes";

export interface UiError {
  code: ExtensionErrorCode | "unknown";
  title: string;
  message: string;
}

function readBackendCode(err: unknown): string | undefined {
  const anyErr = err as {
    response?: { data?: { details?: { code?: string }; message?: string } };
  };
  return anyErr?.response?.data?.details?.code;
}

export function mapExtensionError(err: unknown): UiError {
  const code = readBackendCode(err);
  switch (code) {
    case EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED:
      return {
        code,
        title: "Extension Already Installed",
        message: "This extension is already installed for the current scope.",
      };
    case EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED:
      return {
        code,
        title: "Dependency Blocked",
        message: "Uninstall blocked because other extensions still depend on it.",
      };
    case EXTENSION_ERROR_CODES.MISSING_DEPENDENCY:
      return {
        code,
        title: "Missing Dependency",
        message: "Install blocked: required dependency is missing.",
      };
    case EXTENSION_ERROR_CODES.VERSION_MISMATCH:
      return {
        code,
        title: "Version Mismatch",
        message: "Version constraint not satisfied.",
      };
    case EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE:
      return {
        code,
        title: "Dependency Cycle",
        message: "Dependency graph contains a cycle.",
      };
    case EXTENSION_ERROR_CODES.FORBIDDEN:
      return {
        code,
        title: "Forbidden",
        message: "You do not have permission for this operation.",
      };
    case EXTENSION_ERROR_CODES.NOT_FOUND:
      return {
        code,
        title: "Not Found",
        message: "Requested resource was not found.",
      };
    default:
      return {
        code: "unknown",
        title: "Unexpected Error",
        message: "Please retry or contact an administrator.",
      };
  }
}
