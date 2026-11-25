import { Alert, Button, Card, Space, Tabs, Tooltip } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import { CodeEditor } from '@/components/MonacoDynamic';
import { defineSublimeTheme } from '@/components/JsonCodeEditor';
import type { FormValues } from '@/types/dashboard';
import { primarySchemaType } from '@/utils/schemaDialect';
import type { FormSchemaState } from './types';

interface RequestBodyEditorProps {
  mode: 'form' | 'json';
  rawJson: string;
  formState: FormSchemaState;
  formValues: FormValues;
  formRef: React.RefObject<SchemaFormRendererHandle>;
  onModeChange: (mode: 'form' | 'json') => void;
  /** SUI-018：显式填入示例模板（default > example > enum 首项 > 类型占位），
   * 同时应用到表单与 JSON 编辑器；无 schema（idle/unavailable）时按钮禁用。 */
  onFillExamples: () => void;
  onRawJsonChange: (value: string) => void;
  onFormValuesChange: (values: FormValues) => void;
  onFormat: () => void;
}

/** SUI-017：结构化表单只能表达对象根；根数组/标量不提供表单页签，
 * 编辑与提交都走原始 JSON，保证根值类型经往返不变。 */
function formTabAvailable(formState: FormSchemaState): boolean {
  return (
    formState.status === 'ready' &&
    primarySchemaType(formState.spec.jsonSchema as { type?: string | string[] }) === 'object'
  );
}

export default function RequestBodyEditor(props: RequestBodyEditorProps) {
  const intl = useIntl();
  const formAvailable = formTabAvailable(props.formState);
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'pages.functionsInvoke.requestBody.title',
        defaultMessage: '请求体',
      })}
      extra={
        <Space size={4}>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.functionsInvoke.requestBody.fillExamplesTooltip',
              defaultMessage: '按 Schema 填入完整示例模板（含 example 与占位值），覆盖当前请求体',
            })}
          >
            <Button
              size="small"
              disabled={props.formState.status !== 'ready'}
              onClick={props.onFillExamples}
            >
              <FormattedMessage
                id="pages.functionsInvoke.requestBody.fillExamples"
                defaultMessage="填入示例"
              />
            </Button>
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.functionsInvoke.requestBody.formatTooltip',
              defaultMessage: '将当前请求体格式化',
            })}
          >
            <Button size="small" onClick={props.onFormat}>
              <FormattedMessage
                id="pages.functionsInvoke.requestBody.formatButton"
                defaultMessage="格式化 JSON"
              />
            </Button>
          </Tooltip>
        </Space>
      }
    >
      <Tabs
        activeKey={props.mode}
        onChange={(key) => props.onModeChange(key as 'form' | 'json')}
        items={[
          {
            key: 'json',
            label: intl.formatMessage({
              id: 'pages.functionsInvoke.requestBody.tab.json',
              defaultMessage: '原始 JSON',
            }),
            children: (
              <CodeEditor
                value={props.rawJson}
                onChange={props.onRawJsonChange}
                language="json"
                theme="croupier-sublime"
                beforeMount={defineSublimeTheme}
                height={420}
                options={{
                  lineNumbers: 'on',
                  folding: true,
                  formatOnPaste: true,
                  formatOnType: true,
                  tabSize: 2,
                  scrollBeyondLastLine: false,
                }}
              />
            ),
          },
          ...(formAvailable
            ? [
                {
                  key: 'form',
                  label: intl.formatMessage({
                    id: 'pages.functionsInvoke.requestBody.tab.form',
                    defaultMessage: 'Schema 表单',
                  }),
                  children:
                    props.formState.status === 'ready' ? (
                      <SchemaFormRenderer
                        ref={props.formRef}
                        spec={props.formState.spec}
                        initialValues={props.formValues}
                        onValuesChange={(_, values) => props.onFormValuesChange(values)}
                        hideSubmit
                      />
                    ) : (
                      <Alert
                        type="info"
                        showIcon
                        title={
                          props.formState.status === 'unavailable'
                            ? props.formState.error
                            : intl.formatMessage({
                                id: 'pages.functionsInvoke.requestBody.selectFunctionFirst',
                                defaultMessage: '请选择函数',
                              })
                        }
                        description={intl.formatMessage({
                          id: 'pages.functionsInvoke.requestBody.schemaFallbackDescription',
                          defaultMessage: '原始 JSON 模式始终可用，不依赖 Schema。',
                        })}
                      />
                    ),
                },
              ]
            : []),
        ]}
      />
    </Card>
  );
}
