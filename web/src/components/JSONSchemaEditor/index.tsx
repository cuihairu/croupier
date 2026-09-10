import React, { useState, useCallback, useEffect } from 'react';
import { Card, Input, Button, Space, Divider, Alert, Modal, Dropdown } from 'antd';
import {
  QuestionCircleOutlined,
  SettingOutlined,
  FunctionOutlined,
  AppstoreOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import { jsonParse } from '@/utils/json';
import { SCHEMA_TEMPLATES } from './templates';
import type { JSONSchemaEditorProps, PropertyConfig, SchemaObject } from './types';
import { ObjectPropertyEditor } from './ObjectPropertyEditor';

const { TextArea } = Input;

export default function JSONSchemaEditor({ value, onChange }: JSONSchemaEditorProps) {
  const [activeTab, setActiveTab] = useState<'visual' | 'code'>('visual');
  const [jsonError, setJsonError] = useState<string>('');
  const buildSchema = (input?: SchemaObject): SchemaObject => {
    if (
      !input ||
      typeof input !== 'object' ||
      Array.isArray(input) ||
      Object.keys(input).length === 0
    ) {
      return {
        type: 'object',
        properties: {},
        required: [],
      };
    }
    return input;
  };
  const [schemaData, setSchemaData] = useState<SchemaObject>(buildSchema(value));

  useEffect(() => {
    setSchemaData(buildSchema(value));
    setJsonError('');
  }, [value]);

  const handleVisualChange = useCallback(
    (newData: SchemaObject) => {
      setSchemaData(newData);
      onChange?.(newData);
    },
    [onChange],
  );

  const handleCodeChange = (jsonString: string) => {
    try {
      const parsed = jsonParse(jsonString) as SchemaObject;
      setSchemaData(parsed);
      onChange?.(parsed);
      setJsonError('');
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid JSON';
      setJsonError(message);
    }
  };

  const addCommonProperty = (type: string, preset?: Partial<PropertyConfig>) => {
    const propName = `new${type.charAt(0).toUpperCase() + type.slice(1)}${
      Object.keys(schemaData.properties || {}).length + 1
    }`;
    const newProperty: PropertyConfig = {
      type: type as PropertyConfig['type'],
      title: propName,
      description: `A ${type} property`,
      ...preset,
    };

    handleVisualChange({
      ...schemaData,
      properties: {
        ...schemaData.properties,
        [propName]: newProperty,
      },
    });
  };

  // Load template
  const loadTemplate = (templateKey: string) => {
    const template = SCHEMA_TEMPLATES[templateKey];
    if (template) {
      handleVisualChange({
        type: 'object',
        properties: { ...template.properties },
        required: [...(template.required || [])],
      });
      Modal.success({
        title: 'Template Loaded',
        content: `Loaded "${templateKey}" template. You can now customize it.`,
      });
    }
  };

  return (
    <Card>
      <Space orientation="vertical" style={{ width: '100%' }}>
        <Space>
          <Button.Group>
            <Button icon={<FunctionOutlined />} onClick={() => addCommonProperty('string')}>
              Add String
            </Button>
            <Button icon={<AppstoreOutlined />} onClick={() => addCommonProperty('number')}>
              Add Number
            </Button>
            <Button icon={<SettingOutlined />} onClick={() => addCommonProperty('boolean')}>
              Add Boolean
            </Button>
            <Button icon={<AppstoreOutlined />} onClick={() => addCommonProperty('array')}>
              Add Array
            </Button>
          </Button.Group>

          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: 'player',
                  label: (
                    <Space>
                      <FunctionOutlined /> Player Entity
                    </Space>
                  ),
                  onClick: () => loadTemplate('player'),
                },
                {
                  key: 'item',
                  label: (
                    <Space>
                      <AppstoreOutlined /> Item Entity
                    </Space>
                  ),
                  onClick: () => loadTemplate('item'),
                },
                {
                  key: 'guild',
                  label: (
                    <Space>
                      <SettingOutlined /> Guild Entity
                    </Space>
                  ),
                  onClick: () => loadTemplate('guild'),
                },
                { type: 'divider' as const },
                {
                  key: 'basic',
                  label: (
                    <Space>
                      <FileTextOutlined /> Basic Entity
                    </Space>
                  ),
                  onClick: () => loadTemplate('basic'),
                },
              ],
            }}
          >
            <Button icon={<FileTextOutlined />}>Load Template</Button>
          </Dropdown>

          <div style={{ marginLeft: 'auto' }}>
            <Button.Group>
              <Button
                type={activeTab === 'visual' ? 'primary' : 'default'}
                onClick={() => setActiveTab('visual')}
              >
                Visual Editor
              </Button>
              <Button
                type={activeTab === 'code' ? 'primary' : 'default'}
                onClick={() => setActiveTab('code')}
              >
                JSON Code
              </Button>
            </Button.Group>
          </div>
        </Space>

        <Alert
          message="Schema Editor"
          description="Visual editor allows you to build JSON Schema with a user-friendly interface. Switch to JSON Code for advanced editing."
          type="info"
          showIcon
          icon={<QuestionCircleOutlined />}
        />

        {activeTab === 'visual' ? (
          <div>
            <Divider />
            <ObjectPropertyEditor
              properties={schemaData.properties || {}}
              onChange={(properties) => handleVisualChange({ ...schemaData, properties })}
              required={schemaData.required || []}
              onRequiredChange={(required) => handleVisualChange({ ...schemaData, required })}
            />
          </div>
        ) : (
          <div>
            <TextArea
              value={JSON.stringify(schemaData, null, 2)}
              onChange={(e) => handleCodeChange(e.target.value)}
              rows={20}
              placeholder="Paste or edit JSON Schema here..."
              style={{ fontFamily: 'Monaco, Consolas, monospace' }}
            />
            {jsonError && (
              <Alert
                message="JSON Error"
                description={jsonError}
                type="error"
                showIcon
                style={{ marginTop: 8 }}
              />
            )}
          </div>
        )}
      </Space>
    </Card>
  );
}
