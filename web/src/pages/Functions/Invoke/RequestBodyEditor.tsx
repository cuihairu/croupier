import { Alert, Button, Card, Tabs, Tooltip } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import { CodeEditor } from '@/components/MonacoDynamic';
import type { FormValues } from '@/types/dashboard';
import type { FormSchemaState } from './types';

interface RequestBodyEditorProps {
  mode: 'form' | 'json';
  rawJson: string;
  formState: FormSchemaState;
  formValues: FormValues;
  formRef: React.RefObject<SchemaFormRendererHandle>;
  onModeChange: (mode: 'form' | 'json') => void;
  onRawJsonChange: (value: string) => void;
  onFormValuesChange: (values: FormValues) => void;
  onFormat: () => void;
}

export default function RequestBodyEditor(props: RequestBodyEditorProps) {
  const intl = useIntl();
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'pages.functionsInvoke.requestBody.title',
        defaultMessage: '请求体',
      })}
      extra={
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
                theme="vs-dark"
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
                  message={
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
        ]}
      />
    </Card>
  );
}
