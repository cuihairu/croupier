import React, { useState } from 'react';
import { Input, Button, Space, Collapse, Tag } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import type { PropertyConfig } from './types';
import { InlinePropertyEditor } from './InlinePropertyEditor';

// Object 属性编辑器
export const ObjectPropertyEditor: React.FC<{
  properties: Record<string, PropertyConfig>;
  onChange: (properties: Record<string, PropertyConfig>) => void;
  required: string[];
  onRequiredChange: (required: string[]) => void;
}> = ({ properties, onChange, required, onRequiredChange }) => {
  const [newPropertyName, setNewPropertyName] = useState('');

  const handleAddProperty = () => {
    if (!newPropertyName) return;

    onChange({
      ...properties,
      [newPropertyName]: {
        type: 'string',
        title: newPropertyName,
        description: '',
      },
    });
    setNewPropertyName('');
  };

  const handleDeleteProperty = (property: string) => {
    const newProperties = { ...properties };
    delete newProperties[property];
    onChange(newProperties);
    onRequiredChange(required.filter((p) => p !== property));
  };

  const handleUpdateProperty = (property: string, config: PropertyConfig) => {
    onChange({
      ...properties,
      [property]: config,
    });
  };

  return (
    <div>
      <Space orientation="vertical" style={{ width: '100%' }}>
        <Space>
          <Input
            placeholder="Property name"
            value={newPropertyName}
            onChange={(e) => setNewPropertyName(e.target.value)}
            onPressEnter={handleAddProperty}
          />
          <Button icon={<PlusOutlined />} onClick={handleAddProperty}>
            Add Property
          </Button>
        </Space>

        <Collapse
          ghost
          defaultActiveKey={Object.keys(properties)}
          items={Object.entries(properties).map(([property, config]) => ({
            key: property,
            label: (
              <Space>
                <strong>{property}</strong>
                <Tag
                  color={
                    config.type === 'string'
                      ? 'blue'
                      : config.type === 'number'
                        ? 'green'
                        : config.type === 'boolean'
                          ? 'orange'
                          : config.type === 'array'
                            ? 'purple'
                            : 'geekblue'
                  }
                >
                  {config.type}
                </Tag>
                {required.includes(property) && <Tag color="red">Required</Tag>}
              </Space>
            ),
            children: (
              <InlinePropertyEditor
                property={property}
                config={config}
                onChange={handleUpdateProperty}
                onDelete={handleDeleteProperty}
              />
            ),
          }))}
        />
      </Space>
    </div>
  );
};
