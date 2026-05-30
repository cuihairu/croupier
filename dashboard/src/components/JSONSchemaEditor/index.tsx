import React, { useState, useCallback, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Space,
  Select,
  Switch,
  InputNumber,
  Collapse,
  Divider,
  Alert,
  Tag,
  List,
  Modal,
  Dropdown,
  Menu,
} from 'antd';
import type { CollapseProps } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  CopyOutlined,
  EyeOutlined,
  QuestionCircleOutlined,
  SettingOutlined,
  FunctionOutlined,
  AppstoreOutlined,
  DownOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import type { FormInstance } from 'antd/es/form';
import { jsonParse } from '@/utils/json';

const { TextArea } = Input;
const { Option } = Select;

// Preset templates for common entity types
const SCHEMA_TEMPLATES: Record<string, any> = {
  player: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Player ID', description: 'Unique player identifier' },
      name: { type: 'string', title: 'Name', description: 'Player display name' },
      level: {
        type: 'integer',
        title: 'Level',
        description: 'Player level',
        minimum: 1,
        default: 1,
      },
      experience: {
        type: 'number',
        title: 'Experience',
        description: 'Total experience points',
        default: 0,
      },
      coins: { type: 'integer', title: 'Coins', description: 'In-game currency', default: 0 },
      gems: { type: 'integer', title: 'Gems', description: 'Premium currency', default: 0 },
      isVip: {
        type: 'boolean',
        title: 'VIP Status',
        description: 'Whether player has VIP',
        default: false,
      },
      lastLogin: { type: 'string', title: 'Last Login', format: 'date-time' },
      serverId: { type: 'string', title: 'Server ID', description: 'Home server identifier' },
    },
    required: ['id', 'name'],
  },
  item: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Item ID', description: 'Unique item identifier' },
      name: { type: 'string', title: 'Name', description: 'Item name' },
      type: {
        type: 'string',
        title: 'Type',
        enum: ['weapon', 'armor', 'consumable', 'material', 'special'],
      },
      rarity: {
        type: 'string',
        title: 'Rarity',
        enum: ['common', 'uncommon', 'rare', 'epic', 'legendary'],
      },
      level: { type: 'integer', title: 'Required Level', minimum: 1, default: 1 },
      price: { type: 'number', title: 'Price', description: 'Base price in coins', minimum: 0 },
      stackable: { type: 'boolean', title: 'Stackable', default: false },
      maxStack: { type: 'integer', title: 'Max Stack Size', minimum: 1, default: 99 },
    },
    required: ['id', 'name', 'type'],
  },
  guild: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Guild ID', description: 'Unique guild identifier' },
      name: { type: 'string', title: 'Guild Name', description: 'Display name' },
      leader: { type: 'string', title: 'Leader ID', description: 'Guild leader player ID' },
      level: { type: 'integer', title: 'Guild Level', minimum: 1, default: 1 },
      members: {
        type: 'array',
        title: 'Member List',
        description: 'Array of member IDs',
        items: { type: 'string' },
      },
      maxMembers: { type: 'integer', title: 'Max Members', minimum: 1, default: 50 },
      description: { type: 'string', title: 'Description', description: 'Guild description' },
      created: { type: 'string', title: 'Created At', format: 'date-time' },
    },
    required: ['id', 'name', 'leader'],
  },
  basic: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'ID', description: 'Unique identifier' },
      name: { type: 'string', title: 'Name', description: 'Display name' },
      description: { type: 'string', title: 'Description', description: 'Detailed description' },
      active: { type: 'boolean', title: 'Active', default: true },
    },
    required: ['id', 'name'],
  },
};

interface PropertyConfig {
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array' | 'object';
  title?: string;
  description?: string;
  default?: any;
  enum?: any[];
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  required?: boolean;
  readOnly?: boolean;
  const?: any;
  $ref?: string;
  // Array specific
  items?: PropertyConfig;
  minItems?: number;
  maxItems?: number;
  uniqueItems?: boolean;
  // Object specific
  properties?: Record<string, PropertyConfig>;
  additionalProperties?: boolean | PropertyConfig;
  requiredProperties?: string[];
  // UI specific
  widget?: string;
  placeholder?: string;
  help?: string;
}

interface JSONSchemaEditorProps {
  value?: any;
  onChange?: (value: any) => void;
  schemaFormSchema?: any;
}

// 内联属性编辑器
const InlinePropertyEditor: React.FC<{
  property: string;
  config: PropertyConfig;
  onChange: (property: string, config: PropertyConfig) => void;
  onDelete: (property: string) => void;
}> = ({ property, config, onChange, onDelete }) => {
  const intl = useIntl();

  const updateConfig = (updates: Partial<PropertyConfig>) => {
    onChange(property, { ...config, ...updates });
  };

  return (
    <Card size="small" style={{ marginBottom: 8 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
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
              onChange={(format) => updateConfig({ format: format || null })}
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
              onChange={(e) => updateConfig({ pattern: e.target.value || null })}
              style={{ width: 150 }}
            />
          </Space>
        )}

        {(config.type === 'number' || config.type === 'integer') && (
          <Space wrap>
            <InputNumber
              placeholder="Default"
              value={config.default}
              onChange={(value) => updateConfig({ default: value || null })}
              style={{ width: 120 }}
            />
            <InputNumber
              placeholder="Minimum"
              value={config.minimum}
              onChange={(value) => updateConfig({ minimum: value || undefined })}
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
                updateConfig({ items: type ? { type: type as PropertyConfig['type'] } : null });
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

// Object 属性编辑器
const ObjectPropertyEditor: React.FC<{
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
      <Space direction="vertical" style={{ width: '100%' }}>
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

export default function JSONSchemaEditor({ value, onChange }: JSONSchemaEditorProps) {
  const intl = useIntl();
  const [activeTab, setActiveTab] = useState<'visual' | 'code'>('visual');
  const [jsonError, setJsonError] = useState<string>('');
  const buildSchema = (input?: any) => {
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
  const [schemaData, setSchemaData] = useState<any>(buildSchema(value));

  useEffect(() => {
    setSchemaData(buildSchema(value));
    setJsonError('');
  }, [value]);

  const handleVisualChange = useCallback(
    (newData: any) => {
      setSchemaData(newData);
      onChange?.(newData);
    },
    [onChange],
  );

  const handleCodeChange = (jsonString: string) => {
    try {
      const parsed = jsonParse(jsonString);
      setSchemaData(parsed);
      onChange?.(parsed);
      setJsonError('');
    } catch (error: any) {
      setJsonError(error.message || 'Invalid JSON');
    }
  };

  const addCommonProperty = (type: string, preset?: any) => {
    const propName = `new${type.charAt(0).toUpperCase() + type.slice(1)}${
      Object.keys(schemaData.properties || {}).length + 1
    }`;
    const newProperty: PropertyConfig = {
      type: type as PropertyConfig['type'],
      title: propName,
      description: `A ${type} property`,
      ...(preset || {}),
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
      <Space direction="vertical" style={{ width: '100%' }}>
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
            overlay={
              <Menu>
                <Menu.Item onClick={() => loadTemplate('player')}>
                  <Space>
                    <FunctionOutlined /> Player Entity
                  </Space>
                </Menu.Item>
                <Menu.Item onClick={() => loadTemplate('item')}>
                  <Space>
                    <AppstoreOutlined /> Item Entity
                  </Space>
                </Menu.Item>
                <Menu.Item onClick={() => loadTemplate('guild')}>
                  <Space>
                    <SettingOutlined /> Guild Entity
                  </Space>
                </Menu.Item>
                <Menu.Divider />
                <Menu.Item onClick={() => loadTemplate('basic')}>
                  <Space>
                    <FileTextOutlined /> Basic Entity
                  </Space>
                </Menu.Item>
              </Menu>
            }
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
