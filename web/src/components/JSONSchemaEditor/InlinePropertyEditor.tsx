import React from 'react';
import { Card, Input, Button, Space, Select, InputNumber, Switch } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import type { PropertyConfig } from './types';

const { Option } = Select;

// 内联属性编辑器
export const InlinePropertyEditor: React.FC<{
  property: string;
  config: PropertyConfig;
  onChange: (property: string, config: PropertyConfig) => void;
  onDelete: (property: string) => void;
}> = ({ property, config, onChange, onDelete }) => {
  const updateConfig = (updates: Partial<PropertyConfig>) => {
    onChange(property, { ...config, ...updates });
  };

  return (
    <Card size="small" style={{ marginBottom: 8 }}>
      <Space orientation="vertical" style={{ width: '100%' }}>
        <Space>
          <Input
            placeholder="Property Name"
            value={property}
            onChange={(e) => {
              const newProp = e.target.value;
              onDelete(property);
              onChange(newProp, config);
            }}
            style={{ width: 200 }}
          />
          <Select
            value={config.type}
            onChange={(type) => updateConfig({ type: type as PropertyConfig['type'] })}
            style={{ width: 120 }}
          >
            <Option value="string">String</Option>
            <Option value="number">Number</Option>
            <Option value="integer">Integer</Option>
            <Option value="boolean">Boolean</Option>
            <Option value="array">Array</Option>
            <Option value="object">Object</Option>
          </Select>
          <Input
            placeholder="Title"
            value={config.title || ''}
            onChange={(e) => updateConfig({ title: e.target.value })}
            style={{ width: 150 }}
          />
          <Input
            placeholder="Description"
            value={config.description || ''}
            onChange={(e) => updateConfig({ description: e.target.value })}
            style={{ width: 200 }}
          />
          <Button icon={<DeleteOutlined />} danger onClick={() => onDelete(property)} />
        </Space>

        {/* Type-specific configurations */}
        {config.type === 'string' && (
          <Space wrap>
            <Input
              placeholder="Default"
              value={config.default !== undefined ? String(config.default) : ''}
              onChange={(e) => updateConfig({ default: e.target.value || null })}
              style={{ width: 120 }}
            />
            <Select
              placeholder="Format"
              value={config.format || ''}
              onChange={(format) => updateConfig({ format: format || undefined })}
              style={{ width: 120 }}
            >
              <Option value="date">Date</Option>
              <Option value="date-time">DateTime</Option>
              <Option value="time">Time</Option>
              <Option value="email">Email</Option>
              <Option value="uri">URI</Option>
              <Option value="uuid">UUID</Option>
            </Select>
            <InputNumber
              placeholder="Min Length"
              value={config.minLength}
              onChange={(value) => updateConfig({ minLength: value || undefined })}
              style={{ width: 100 }}
              min={0}
            />
            <InputNumber
              placeholder="Max Length"
              value={config.maxLength}
              onChange={(value) => updateConfig({ maxLength: value || undefined })}
              style={{ width: 100 }}
              min={0}
            />
            <Input
              placeholder="Pattern"
              value={config.pattern || ''}
              onChange={(e) => updateConfig({ pattern: e.target.value || undefined })}
              style={{ width: 150 }}
            />
          </Space>
        )}

        {(config.type === 'number' || config.type === 'integer') && (
          <Space wrap>
            <InputNumber
              placeholder="Default"
              value={typeof config.default === 'number' ? config.default : undefined}
              onChange={(value) => updateConfig({ default: value ?? null })}
              style={{ width: 120 }}
            />
            <InputNumber
              placeholder="Minimum"
              value={config.minimum}
              onChange={(value) => updateConfig({ minimum: value ?? undefined })}
              style={{ width: 100 }}
            />
            <InputNumber
              placeholder="Maximum"
              value={config.maximum}
              onChange={(value) => updateConfig({ maximum: value || undefined })}
              style={{ width: 100 }}
            />
          </Space>
        )}

        {config.type === 'boolean' && (
          <Space>
            <Select
              placeholder="Default"
              value={config.default === true ? 'true' : config.default === false ? 'false' : ''}
              onChange={(value) => {
                if (value === 'true') updateConfig({ default: true });
                else if (value === 'false') updateConfig({ default: false });
                else updateConfig({ default: null });
              }}
              style={{ width: 120 }}
            >
              <Option value="true">True</Option>
              <Option value="false">False</Option>
            </Select>
          </Space>
        )}

        {config.type === 'array' && (
          <Space wrap>
            <Select
              placeholder="Items Type"
              value={config.items?.type}
              onChange={(type) => {
                updateConfig({
                  items: type ? { type: type as PropertyConfig['type'] } : undefined,
                });
              }}
              style={{ width: 120 }}
            >
              <Option value="string">String</Option>
              <Option value="number">Number</Option>
              <Option value="integer">Integer</Option>
              <Option value="boolean">Boolean</Option>
              <Option value="object">Object</Option>
            </Select>
            <InputNumber
              placeholder="Min Items"
              value={config.minItems}
              onChange={(value) => updateConfig({ minItems: value || undefined })}
              style={{ width: 100 }}
              min={0}
            />
            <InputNumber
              placeholder="Max Items"
              value={config.maxItems}
              onChange={(value) => updateConfig({ maxItems: value || undefined })}
              style={{ width: 100 }}
              min={0}
            />
            <Switch
              checked={config.uniqueItems || false}
              onChange={(uniqueItems) => updateConfig({ uniqueItems })}
              checkedChildren="Unique Items"
            />
          </Space>
        )}

        {config.type === 'object' && (
          <Space wrap>
            <Switch
              checked={config.additionalProperties === true || false}
              onChange={(additionalProperties) => {
                updateConfig({
                  additionalProperties,
                  ...(additionalProperties ? {} : { additionalProperties: false }),
                });
              }}
              checkedChildren="Additional Properties"
            />
          </Space>
        )}

        <Space wrap>
          <Switch
            checked={config.required || false}
            onChange={(required) => updateConfig({ required })}
            checkedChildren="Required"
          />
          <Switch
            checked={config.readOnly || false}
            onChange={(readOnly) => updateConfig({ readOnly })}
            checkedChildren="Read Only"
          />
        </Space>
      </Space>
    </Card>
  );
};
