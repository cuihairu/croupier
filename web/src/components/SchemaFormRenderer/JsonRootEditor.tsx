/**
 * SUI-026：非对象根请求的 JSON 编辑（生成页面与组合页区块共用）。
 *
 * 结构化控件只能表达对象根；契约 schema 声明根数组/标量/null 时（SUI-011
 * 请求兼容性分级的 degraded 形态），SchemaFormRenderer 经本组件提供 JSON
 * 编辑入口：与对象表单同一份 handle 契约（validate/getSubmitValues/submit）、
 * 同一 AJV 校验器、同一绑定执行链——payload 原样提交（数组/标量不套对象壳，
 * SUI-017 往返语义）。界面同时给出不能渲染结构化表单的具体原因（根类型）
 * 与实际可用入口（本编辑器）。
 */
import React, { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react';
import { Alert, Button, Input } from 'antd';
import { getIntl } from '@umijs/max';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema } from '@rjsf/utils';
import { deriveFormDefaults } from '@/utils/json';
import { primarySchemaType } from '@/utils/schemaDialect';
import type { FormValues, JSONValue, JSONSchema, FormPresentationSpec } from '@/types/dashboard';
import type { SchemaFormRendererHandle } from './index';

interface JsonRootEditorProps {
  spec: FormPresentationSpec;
  initialValues?: FormValues;
  onFinish?: (values: FormValues) => Promise<boolean | void> | boolean | void;
  onValuesChange?: (changedValues: FormValues, allValues: FormValues) => void;
  readonly?: boolean;
  disabled?: boolean;
  hideSubmit?: boolean;
}

/** schema 节点本身是否为对象形态（可读 type 等关键字）。 */
function isSchemaNode(
  value: JSONSchema | undefined | null,
): value is JSONSchema & Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

/** SUI-026：声明了非 object 根类型（数组/标量/null）→ JSON 编辑形态。
 * 未声明 type 的 schema 不进本形态（保持对象表单路径与 SUI-025 兜底不变）。 */
export function isJsonRootSpec(schema?: JSONSchema | null): boolean {
  if (!isSchemaNode(schema)) return false;
  const root = primarySchemaType(schema as { type?: string | string[] });
  return root !== undefined && root !== 'object';
}

/** 根类型的展示名（本地化 reason 的 {type} 参数）。 */
export function jsonRootTypeName(schema: Record<string, unknown>): string {
  const root = primarySchemaType(schema as { type?: string | string[] });
  return root ?? 'unknown';
}

function fmt(id: string, defaultMessage: string, values?: Record<string, string | number>): string {
  return getIntl().formatMessage({ id, defaultMessage }, values);
}

const JsonRootEditor = React.forwardRef<SchemaFormRendererHandle, JsonRootEditorProps>(
  ({ spec, initialValues, onFinish, onValuesChange, readonly = false, disabled = false, hideSubmit = false }, ref) => {
    const schema = spec.jsonSchema;
    // 种子：显式 initialValues 优先；否则 SUI-018 默认值层（含根类型骨架，
    // SUI-017：根数组 `[]`/items 骨架、标量类型占位）。调用方常用 `|| {}`
    // 兜底（CompositeRenderer/ResourcePageRenderer 的 sectionInputs 等）——
    // 对象兜底对非对象根永远不是合法种子，忽略并回退 schema 派生。
    const seed = useMemo<JSONValue>(() => {
      if (
        initialValues !== undefined &&
        initialValues !== null &&
        !(typeof initialValues === 'object' && !Array.isArray(initialValues))
      ) {
        return initialValues as JSONValue;
      }
      return (deriveFormDefaults(schema as never) ?? null) as JSONValue;
      // eslint-disable-next-line react-hooks/exhaustive-deps -- 种子只随初值/schema 内容变化，见下方 key
    }, [initialValues, spec]);
    const seedText = useMemo(() => JSON.stringify(seed, null, 2), [seed]);

    const [text, setText] = useState<string>(seedText);
    const [parseError, setParseError] = useState<string | null>(null);
    const [schemaErrors, setSchemaErrors] = useState<string[]>([]);
    // 最后一次成功解析的值（getSubmitValues 的返回源；解析失败时保持旧值）
    const parsedRef = useRef<JSONValue>(seed);
    const textRef = useRef<string>(seedText);
    textRef.current = text;

    // 外部重置初值（JSON 变化）或换 spec（同 key 语义）才回种；自身编辑回声跳过
    const reseedKey = useMemo(
      () => `${JSON.stringify(seed)}|${JSON.stringify(schema ?? null)}`,
      [seed, schema],
    );
    const lastReseedRef = useRef<string | null>(null);
    useEffect(() => {
      if (lastReseedRef.current === reseedKey) return;
      lastReseedRef.current = reseedKey;
      setText(seedText);
      setParseError(null);
      setSchemaErrors([]);
      parsedRef.current = seed;
    }, [reseedKey, seedText, seed]);

    /** 解析当前文本；失败返回 null 并落解析错误。 */
    const parseCurrent = useCallback((): JSONValue | null => {
      try {
        const parsed = JSON.parse(textRef.current) as JSONValue;
        setParseError(null);
        return parsed;
      } catch (err) {
        setParseError(
          fmt(
            'component.schemaFormRenderer.jsonRoot.invalidJson',
            'JSON 解析失败：{message}',
            {
              message: err instanceof Error ? err.message : String(err),
            },
          ),
        );
        return null;
      }
    }, []);

    const validate = useCallback((): boolean => {
      const parsed = parseCurrent();
      if (parsed === null) return false;
      parsedRef.current = parsed;
      // 与对象表单同一校验器（@rjsf/validator-ajv8，SUI-016 服务端同源策略）
      const result = validator.validateFormData(parsed, (schema ?? {}) as RJSFSchema);
      if (!result.errors.length) {
        setSchemaErrors([]);
        return true;
      }
      const intl = getIntl();
      const mismatchTitle = intl.formatMessage({
        id: 'component.schemaFormRenderer.jsonRoot.schemaMismatch',
        defaultMessage: '请求 JSON 不符合 schema',
      });
      setSchemaErrors([
        mismatchTitle,
        ...result.errors.map((error) => `${error.property || 'root'}: ${error.message}`),
      ]);
      return false;
    }, [parseCurrent, schema]);

    const getSubmitValues = useCallback((): FormValues => {
      // 有意跨越：FormValues 按对象根声明为 Record；JSON 根的整值即请求
      // payload（数组/标量），运行时保持 JSONValue 原样，服务端按契约 schema
      // 校验（SUI-016）。此处仅在 JSON 根形态下发生。
      return parsedRef.current as FormValues;
    }, []);

    const submit = useCallback((): void => {
      if (!validate()) return;
      void onFinish?.(getSubmitValues());
    }, [validate, onFinish, getSubmitValues]);

    useImperativeHandle(ref, () => ({ submit, validate, getValues: getSubmitValues, getSubmitValues }), [
      submit,
      validate,
      getSubmitValues,
    ]);

    const handleChange = useCallback(
      (event: React.ChangeEvent<HTMLTextAreaElement>): void => {
        const next = event.target.value;
        setText(next);
        try {
          const parsed = JSON.parse(next) as JSONValue;
          parsedRef.current = parsed;
          setParseError(null);
          onValuesChange?.({}, parsed as FormValues);
        } catch {
          // 编辑中间态允许非法 JSON：保持旧解析值，不中断输入
        }
      },
      [onValuesChange],
    );

    const reason = fmt(
      'component.schemaFormRenderer.jsonRoot.reason',
      '该请求是 {type} 类型，结构化表单无法表达；请编辑完整请求 JSON——提交仍经本页面绑定的同一执行与 schema 校验链。',
      {
        type: isSchemaNode(schema) ? jsonRootTypeName(schema) : 'unknown',
      },
    );
    const title = fmt('component.schemaFormRenderer.jsonRoot.title', '请求 JSON');

    return (
      <div className="schema-form-json-root">
        <div className="schema-form-json-root__title">{title}</div>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={reason}
          data-testid="json-root-reason"
        />
        <Input.TextArea
          value={text}
          onChange={handleChange}
          readOnly={readonly}
          disabled={disabled}
          rows={8}
          style={{ fontFamily: 'monospace' }}
          data-testid="json-root-editor"
        />
        {parseError && (
          <Alert type="error" showIcon style={{ marginTop: 12 }} message={parseError} />
        )}
        {schemaErrors.length > 0 && (
          <Alert
            type="error"
            showIcon
            style={{ marginTop: 12 }}
            message={schemaErrors[0]}
            description={
              <ul style={{ margin: 0, paddingLeft: 16 }}>
                {schemaErrors.slice(1).map((item, index) => (
                  <li key={index}>{item}</li>
                ))}
              </ul>
            }
          />
        )}
        {!hideSubmit && (
          <Button
            type="primary"
            style={{ marginTop: 12 }}
            disabled={disabled}
            onClick={submit}
            data-testid="json-root-submit"
          >
            {fmt('component.schemaFormRenderer.submitFallback', '提交')}
          </Button>
        )}
      </div>
    );
  },
);

JsonRootEditor.displayName = 'JsonRootEditor';

export default JsonRootEditor;
