/**
 * 函数调用服务
 */

import { invokeFunction as apiInvokeFunction } from './api/functions';
import type { JSONValue } from '@/types/dashboard';

export interface InvokeFunctionOptions {
  timeout?: number;
  signal?: AbortSignal;
}

export interface InvokeFunctionResult<T = unknown> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
}

/**
 * 调用函数
 */
export async function invokeFunction<T = unknown>(
  functionId: string,
  params: JSONValue,
  options?: InvokeFunctionOptions,
): Promise<T> {
  try {
    if (options?.signal?.aborted) {
      throw new DOMException('函数调用已取消', 'AbortError');
    }
    const result = await apiInvokeFunction(functionId, params);
    return result as T;
  } catch (error: unknown) {
    console.error(`[FunctionInvoke] ${functionId} failed:`, error);
    throw error;
  }
}

/**
 * 批量调用函数
 */
export async function invokeFunctions(
  calls: Array<{ functionId: string; params: JSONValue }>,
): Promise<unknown[]> {
  const results = await Promise.allSettled(
    calls.map((call) => invokeFunction(call.functionId, call.params)),
  );

  return results.map((result, index) => {
    if (result.status === 'fulfilled') {
      return result.value;
    } else {
      console.error(`[FunctionInvoke] Batch call ${index} failed:`, result.reason);
      return null;
    }
  });
}

/**
 * 创建可取消的函数调用
 */
export function createCancellableInvoke<T = unknown>(
  functionId: string,
  params: JSONValue,
  options?: Omit<InvokeFunctionOptions, 'signal'>,
) {
  const controller = new AbortController();

  const promise = invokeFunction<T>(functionId, params, {
    ...options,
    signal: controller.signal,
  });

  return {
    promise,
    cancel: () => controller.abort(),
  };
}

/**
 * 函数调用日志
 */
export interface FunctionInvokeLog {
  functionId: string;
  params: JSONValue;
  result?: unknown;
  error?: unknown;
  duration: number;
  timestamp: number;
}

const invokeLogs: FunctionInvokeLog[] = [];
const MAX_LOGS = 100;

/**
 * 记录函数调用
 */
export function logFunctionInvoke(log: FunctionInvokeLog) {
  invokeLogs.unshift(log);
  if (invokeLogs.length > MAX_LOGS) {
    invokeLogs.pop();
  }
}

/**
 * 获取调用日志
 */
export function getFunctionInvokeLogs(limit = 20): FunctionInvokeLog[] {
  return invokeLogs.slice(0, limit);
}

/**
 * 清除调用日志
 */
export function clearFunctionInvokeLogs() {
  invokeLogs.length = 0;
}

/**
 * 带日志的函数调用
 */
export async function invokeWithLog<T = unknown>(
  functionId: string,
  params: JSONValue,
  options?: InvokeFunctionOptions,
): Promise<T> {
  const startTime = Date.now();

  try {
    const result = await invokeFunction<T>(functionId, params, options);
    const duration = Date.now() - startTime;

    logFunctionInvoke({
      functionId,
      params,
      result,
      duration,
      timestamp: startTime,
    });

    return result;
  } catch (error) {
    const duration = Date.now() - startTime;

    logFunctionInvoke({
      functionId,
      params,
      error,
      duration,
      timestamp: startTime,
    });

    throw error;
  }
}
