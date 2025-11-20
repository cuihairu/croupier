import React, { useEffect, useState } from 'react';
import { Card, Modal } from 'antd';
import { ProColumns, PageContainer } from '@ant-design/pro-components';
import GameSelector from '@/components/GameSelector';
import XResourceTable from '@/components/XResourceTable';
import XEntityForm from '@/components/XEntityForm';
import {
  listEntities,
  createEntity,
  updateEntity,
  deleteEntity,
  validateEntity,
  previewEntity,
  EntityDefinition,
} from '@/services/croupier';

export default function EntitiesPage() {
  const [entities, setEntities] = useState<EntityDefinition[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [editingEntity, setEditingEntity] = useState<EntityDefinition | null>(null);
  const [previewEntity, setPreviewEntity] = useState<EntityDefinition | null>(null);
  const [previewContent, setPreviewContent] = useState<string>('');

  const loadEntities = async () => {
    setLoading(true);
    try {
      const gameId = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      const result = await listEntities({ game_id: gameId, env });
      setEntities(result);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadEntities();
  }, []);

  const handleCreate = () => {
    setEditingEntity(null);
    setModalVisible(true);
  };

  const handleEdit = (entity: EntityDefinition) => {
    setEditingEntity(entity);
    setModalVisible(true);
  };

  const handleDelete = async (entity: EntityDefinition) => {
    const gameId = localStorage.getItem('game_id') || undefined;
    const env = localStorage.getItem('env') || undefined;
    await deleteEntity(entity.id, { game_id: gameId, env });
    loadEntities();
  };

  const handlePreview = async (entity: EntityDefinition) => {
    try {
      const gameId = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      const result = await previewEntity(entity.id, { game_id: gameId, env });
      setPreviewEntity(entity);
      setPreviewContent(result.preview_html || 'No preview available');
      setPreviewVisible(true);
    } catch (error: any) {
      console.error('Failed to load preview:', error);
    }
  };

  const handleSubmit = async (data: any) => {
    const gameId = localStorage.getItem('game_id') || undefined;
    const env = localStorage.getItem('env') || undefined;

    const entityData = {
      name: data.name,
      description: data.description,
      schema: data.schema,
      uiSchema: data.uiSchema,
      operations: data.operations?.split(',').map((op: string) => op.trim()).filter(Boolean) || [],
    };

    if (editingEntity) {
      await updateEntity(editingEntity.id, entityData, { game_id: gameId, env });
    } else {
      await createEntity(entityData, { game_id: gameId, env });
    }

    setModalVisible(false);
    loadEntities();
  };

  const handleValidate = async (data: any) => {
    const gameId = localStorage.getItem('game_id') || undefined;
    const env = localStorage.getItem('env') || undefined;

    const entityData = {
      name: data.name,
      description: data.description,
      schema: data.schema,
      uiSchema: data.uiSchema,
      operations: data.operations?.split(',').map((op: string) => op.trim()).filter(Boolean) || [],
    };

    return await validateEntity(entityData, { game_id: gameId, env });
  };

  const columns: ProColumns<EntityDefinition>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 200,
      ellipsis: true,
    },
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      ellipsis: true,
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Operations',
      dataIndex: 'operations',
      key: 'operations',
      width: 150,
      render: (operations: string[] | string | undefined) => {
        if (Array.isArray(operations)) {
          return operations.length ? operations.join(', ') : '-';
        }
        if (typeof operations === 'string') {
          return operations || '-';
        }
        return '-';
      },
    },
    {
      title: 'Updated',
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 120,
      render: (date: string) => date ? new Date(date).toLocaleDateString() : '-',
    },
  ];

  // Basic JSON Schema for entity schema editing
  const schemaFormSchema = {
    type: 'object',
    properties: {
      type: { type: 'string', enum: ['object'], default: 'object' },
      properties: {
        type: 'object',
        additionalProperties: {
          type: 'object',
          properties: {
            type: { type: 'string', enum: ['string', 'number', 'integer', 'boolean', 'array', 'object'] },
            description: { type: 'string' },
          }
        }
      },
      required: { type: 'array', items: { type: 'string' } },
    }
  };

  return (
    <PageContainer>
      <Card title="Entity Management" extra={<GameSelector />}>
      <XResourceTable<EntityDefinition>
        dataSource={entities}
        loading={loading}
        rowKey="id"
        columns={columns}
        onAdd={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        onPreview={handlePreview}
        addButtonText="New Entity"
        deleteConfirmTitle="Delete Entity"
        getDeleteConfirmContent={(entity) => `Are you sure you want to delete entity "${entity.name || entity.id}"?`}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />

      <XEntityForm<EntityDefinition>
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        entity={editingEntity}
        onSubmit={handleSubmit}
        onValidate={handleValidate}
        enableSchemaEditing={true}
        schemaFormSchema={schemaFormSchema}
        basicFields={[
          {
            name: 'name',
            label: 'Name',
            required: true,
            placeholder: 'Entity display name',
          },
          {
            name: 'description',
            label: 'Description',
            type: 'textarea',
            placeholder: 'Entity description',
          },
          {
            name: 'operations',
            label: 'Operations',
            type: 'array',
            placeholder: 'create, read, update, delete',
          },
        ]}
        getInitialValues={(entity) => ({
          name: entity.name,
          description: entity.description,
          operations: entity.operations?.join(', ') || '',
        })}
        getSchemaData={(entity) => entity.schema || {}}
        getUiSchemaData={(entity) => entity.uiSchema || {}}
      />

      {/* Preview Modal */}
      <Modal
        title={`Preview: ${previewEntity?.name || previewEntity?.id}`}
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        width={800}
        footer={null}
      >
        <div dangerouslySetInnerHTML={{ __html: previewContent }} />
      </Modal>
      </Card>
    </PageContainer>
  );
}
