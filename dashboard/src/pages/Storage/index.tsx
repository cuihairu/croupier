import React, { useEffect, useState, useRef } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Upload,
  Modal,
  App,
  Popconfirm,
  Input,
  Tag,
  Tooltip,
  Image,
  Row,
  Col,
  Statistic,
  Breadcrumb,
  Typography,
  Progress,
} from 'antd';
import {
  UploadOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  LinkOutlined,
  CloudUploadOutlined,
  FileOutlined,
  FolderOutlined,
  FolderAddOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import type { UploadFile } from 'antd/es/upload/interface';
import * as storageAPI from '@/services/api/storage';

const { Text } = Typography;

interface FileInfo {
  key: string;
  name: string;
  size: number;
  lastModified: string;
  url?: string;
  isDirectory?: boolean; // 是否是目录
}

export default function StoragePage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [uploadVisible, setUploadVisible] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [createDirVisible, setCreateDirVisible] = useState(false);
  const [dirName, setDirName] = useState('');
  const [previewFile, setPreviewFile] = useState<{ url: string; name: string }>({
    url: '',
    name: '',
  });
  const [searchKeyword, setSearchKeyword] = useState('');
  const [currentPrefix, setCurrentPrefix] = useState(''); // 当前目录路径
  const [fileUploadList, setFileUploadList] = useState<UploadFile[]>([]);
  const [dirUploadList, setDirUploadList] = useState<UploadFile[]>([]);
  const [uploadStats, setUploadStats] = useState({
    total: 0,
    success: 0,
    failed: 0,
    failedFiles: [] as string[],
  });

  const loadFiles = async () => {
    setLoading(true);
    try {
      const result = await storageAPI.listObjects({
        prefix: currentPrefix || undefined,
        delimiter: '/',
      });
      const items: FileInfo[] = [];

      (result.prefixes || []).forEach((prefix: string) => {
        const displayName = prefix.replace(currentPrefix, '').replace(/\/$/, '');
        if (displayName) {
          items.push({
            key: prefix,
            name: displayName,
            size: 0,
            lastModified: '',
            isDirectory: true,
          });
        }
      });

      (result.objects || []).forEach((obj: any) => {
        if (obj?.key && !obj.key.endsWith('/')) {
          const displayName = obj.key.replace(currentPrefix, '');
          items.push({
            key: obj.key,
            name: displayName,
            size: Number(obj.size || 0),
            lastModified: obj.last_modified || '',
            url: obj.key,
            isDirectory: false,
          });
        }
      });

      setFiles(items);
    } catch (error) {
      message.error('加载文件列表失败');
      setFiles([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFiles();
  }, [currentPrefix]); // 当目录变化时重新加载

  // 进入目录
  const handleEnterDirectory = (directory: FileInfo) => {
    if (directory.isDirectory) {
      setCurrentPrefix(directory.key);
    }
  };

  // 返回上级目录
  const handleGoBack = () => {
    if (currentPrefix) {
      const parts = currentPrefix.split('/').filter((p) => p);
      parts.pop(); // 移除当前目录
      const newPrefix = parts.join('/');
      setCurrentPrefix(newPrefix ? newPrefix + '/' : '');
    }
  };

  // 导航到指定路径的辅助函数
  const navigateToPath = (targetPath: string) => {
    // 确保 path 末尾有斜杠（如果非空）
    const normalizedPath = targetPath
      ? targetPath.endsWith('/')
        ? targetPath
        : targetPath + '/'
      : '';
    setCurrentPrefix(normalizedPath);
  };

  // 获取面包屑路径
  const breadcrumbItems = React.useMemo(() => {
    const items = [
      {
        title: (
          <span style={{ cursor: 'pointer', color: '#1890ff' }} onClick={() => navigateToPath('')}>
            根目录
          </span>
        ),
      },
    ];

    if (currentPrefix) {
      const parts = currentPrefix.split('/').filter((p) => p);
      // 计算每个层级的完整路径
      const paths: string[] = [];
      for (let i = 0; i < parts.length; i++) {
        const path = parts.slice(0, i + 1).join('/') + '/';
        paths.push(path);
      }

      parts.forEach((part, index) => {
        const targetPath = paths[index];
        items.push({
          title: (
            <span
              style={{ cursor: 'pointer', color: '#1890ff' }}
              onClick={() => navigateToPath(targetPath)}
            >
              {part}
            </span>
          ),
        });
      });
    }

    return items;
  }, [currentPrefix]);

  const handleUpload = async (options: any) => {
    const { file, onSuccess, onError, onProgress } = options;

    try {
      const formData = new FormData();

      // 计算文件的完整路径
      let filePath = file.name;

      // 如果是从文件夹选择的文件
      if (file.webkitRelativePath) {
        filePath = file.webkitRelativePath;
      }

      // 如果在子目录中，添加当前目录前缀
      if (currentPrefix) {
        filePath = `${currentPrefix}${filePath}`;
      }

      // 创建新文件，包含完整路径
      const newFile = new File([file], filePath, {
        type: file.type,
        lastModified: file.lastModified,
      });

      formData.append('file', newFile);

      // 添加路径字段，因为 header.Filename 只包含文件名，不包含路径
      formData.append('path', filePath);

      console.log('准备上传文件:', filePath);

      const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';

      // 使用 XMLHttpRequest 来支持进度监控
      const xhr = new XMLHttpRequest();

      // 监听上传进度
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100);
          onProgress({ percent });
        }
      });

      // 监听上传完成
      xhr.addEventListener('load', () => {
        if (xhr.status === 200) {
          try {
            const data = JSON.parse(xhr.responseText);

            // 更新成功统计
            setUploadStats((prev) => ({
              ...prev,
              success: prev.success + 1,
            }));

            onSuccess?.(data);
            checkAllUploadsComplete();
          } catch (error) {
            onError?.(error);
          }
        } else {
          try {
            const errorData = JSON.parse(xhr.responseText);
            const errorMsg = errorData.message || '上传失败';

            // 失败立即提示并记录
            message.error(`${filePath} 上传失败: ${errorMsg}`);
            setUploadStats((prev) => ({
              ...prev,
              failed: prev.failed + 1,
              failedFiles: [...prev.failedFiles, filePath],
            }));

            onError?.(new Error(errorMsg));
          } catch {
            message.error(`${filePath} 上传失败`);
            setUploadStats((prev) => ({
              ...prev,
              failed: prev.failed + 1,
              failedFiles: [...prev.failedFiles, filePath],
            }));
            onError?.(new Error('上传失败'));
          }
        }
      });

      // 监听上传错误
      xhr.addEventListener('error', () => {
        message.error(`${filePath} 上传失败: 网络错误`);
        setUploadStats((prev) => ({
          ...prev,
          failed: prev.failed + 1,
          failedFiles: [...prev.failedFiles, filePath],
        }));
        onError?.(new Error('网络错误'));
      });

      // 发送请求
      xhr.open('POST', '/api/v1/storage/objects');
      if (token) {
        xhr.setRequestHeader('Authorization', `Bearer ${token}`);
      }
      xhr.send(formData);
    } catch (error) {
      message.error(
        `${file.name} 上传失败: ${error instanceof Error ? error.message : '未知错误'}`,
      );
      setUploadStats((prev) => ({
        ...prev,
        failed: prev.failed + 1,
        failedFiles: [...prev.failedFiles, file.name],
      }));
      onError?.(error);
    }
  };

  // 检查所有上传是否完成
  const checkAllUploadsComplete = () => {
    const allFiles = [...fileUploadList, ...dirUploadList];
    const completedCount = allFiles.filter((f) => f.status === 'done').length;

    if (completedCount === allFiles.length && allFiles.length > 0) {
      // 所有上传完成，显示汇总
      const { success, failed, failedFiles } = uploadStats;
      const total = success + failed;

      if (total > 0 && total === allFiles.length) {
        // 只在所有文件都处理完后才刷新
        setTimeout(() => {
          if (failed === 0) {
            message.success(`全部上传完成！共 ${total} 个文件`);
          } else {
            message.warning(
              `上传完成：成功 ${success} 个，失败 ${failed} 个` +
                (failedFiles.length > 0
                  ? `\n失败文件：${failedFiles.slice(0, 3).join(', ')}${
                      failedFiles.length > 3 ? '...' : ''
                    }`
                  : ''),
            );
          }

          // 刷新文件列表
          loadFiles();

          // 重置统计
          setUploadStats({
            total: 0,
            success: 0,
            failed: 0,
            failedFiles: [],
          });
        }, 300);
      }
    }
  };

  const handleDelete = async (key: string) => {
    try {
      await storageAPI.deleteObject(key);
      message.success('删除成功');
      loadFiles();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleBatchDelete = async () => {
    if (selectedKeys.length === 0) {
      message.warning('请先选择要删除的文件');
      return;
    }

    try {
      await storageAPI.batchDeleteObjects(selectedKeys);
      message.success(`成功删除 ${selectedKeys.length} 个文件`);
      setSelectedKeys([]);
      loadFiles();
    } catch (error) {
      message.error('批量删除失败');
    }
  };

  const handleCreateDirectory = async () => {
    if (!dirName.trim()) {
      message.warning('请输入目录名称');
      return;
    }

    try {
      const prefix = currentPrefix ? `${currentPrefix}${dirName.trim()}` : dirName.trim();
      await storageAPI.createDirectory(prefix);
      message.success('目录创建成功');
      setDirName('');
      setCreateDirVisible(false);
      loadFiles();
    } catch (error) {
      message.error('创建目录失败');
    }
  };

  const handlePreview = async (file: FileInfo) => {
    try {
      const data = await storageAPI.getSignedUrl(file.key);
      if (data.url) {
        setPreviewFile({ url: data.url, name: file.name });
        setPreviewVisible(true);
      } else {
        message.error('获取预览链接失败');
      }
    } catch (error) {
      message.error('预览失败');
    }
  };

  const copyLink = async (file: FileInfo) => {
    try {
      const data = await storageAPI.getSignedUrl(file.key);
      if (data.url) {
        await navigator.clipboard.writeText(data.url);
        message.success('链接已复制到剪贴板');
      } else {
        message.error('获取链接失败');
      }
    } catch (error) {
      message.error('复制链接失败');
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  };

  const isImageFile = (filename: string): boolean => {
    const ext = filename.toLowerCase().split('.').pop();
    return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext || '');
  };

  const filteredFiles = files.filter((file) =>
    file.name.toLowerCase().includes(searchKeyword.toLowerCase()),
  );

  const columns: ColumnsType<FileInfo> = [
    {
      title: '文件名',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (text, record) => (
        <Space>
          {record.isDirectory ? (
            <FolderOutlined
              style={{
                fontSize: 24,
                color: '#1890ff',
                cursor: 'pointer',
              }}
              onClick={() => handleEnterDirectory(record)}
            />
          ) : isImageFile(text) ? (
            <Image
              width={40}
              height={40}
              src={record.url}
              style={{ objectFit: 'cover', borderRadius: 4 }}
              fallback="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAMIAAADDCAYAAADQvc6UAAABRWlDQ1BJQ0MgUHJvZmlsZQAAKJF9kT1Iw0AcxV9TpVUqDnYQ6ChOlkQFXGUKhbBQmkrtOpgcukXNGlIUlwcBdeCgx+LVQcXZ10dXAVB8APEydFJ0UVK/F9SaBHjwXE/3t173L0DhHqZqVUptdXL9b2NdI8yg8Xus7BNPvB+0s/eW8+l+EJEswUBHyG+iCC+LwFXrXJhCkijKwoI6dCQbMRwS1nGkYwGDYliYGbEAfCQ5HRQBwHBZj2Sp8ygYgZCiiYLX5uiRNcoOQlhXck6cX580PylUnEhRGNFoFIsaJhGwIgM5qQRklMYJqGPpRnkCQ5oPXP5wg+BxWJtmvfisYzBwDpSE1YwSMA0fGRhxmFkzlMkYfpUCsL2fiFwY6FehxwVROQXl2XgIFlB8K2JXW4xGB3WrXWxUXMTY/aTKImhUD/Zk3zNtKZXQPPJT+Ys4KFYhVPkPi8tViKqDcVi6RqyRBGuPxG5xi1RiKqLTB4yHvL7y5OqWXLUyERxCa0UAM0zR4MND0/wR4MLy6tEE8yDcIGXIM1dvw4ThKS/wR4nYiO0K9M5eBgkIuR/N2uNlpxSqPtVzdPPXpDw8N/ju+dJ+4v4Ua/G/yfBDfzG0RNnggYAlBlPPK7vJEb/kJCx+XBbB9wX8R4oU8p0AAAAJcEhZcwAADsMAAA7DAcdvqGQAAAMaSURBVHja7d0LbBRHFMR/vu9lJtESQIqUEUKECQSQTvffAYqnpI4oJffHQTBRc8HAoKBX8CgeHgKKiIyirhwouKQkHTvXRImTdjN7uzsZu7O6NH5p4aTM2f2mTNn9u6b0qlGo9FoNIQ/0FUqVRKJRNLo6+oqpVQqjUajtZvnUim1Wq1UKkqlSk9HUqm0VCr1er1er7+e25/sipKcnp6oqGqa1k3TdG0tqnrTWmuttbU1NTXZ8/1vWmut7d27d3l5eTqdpqnm6ZomSTrOuCqnKMrzvKOqao6IMsyyrK3RaHTT46/n+yyKyKLn+EQqleF4wXCZHiBCCiqVAcs80yyzKsimEQs4loiwphFBaK2VUnY4wVgxhRWiml7hABdF8aZpmmWcSwRxpLKqUQqGcMcaA1tZSa+1hjDWOqqrL8qcpSyllCKAUhZpQSRhShrbWutAaXUWmtba533lFKYUsoYQoQQhZbRWmtqa8pFSnlFKYUQgghpJRSCmFk/Kr89Pz85PnOCmlCmGM8Szn/fqf//Kfn+lfp1RXk1oSyrlzKf2fY2y2X2oFyhvvXvvb4T34JRjY4wJdyil1FrrWmtvbGwcHR2d7z3zHvPEsm1ba63dbjdMT0+r1SqnDJIZpa2aJlmWNMdYYKaWWA1hba2WwAAghrJQy1lrnAHD0aJrvfW+/wtAfvuYrd/8bAAAAAElFTkSuQmCC"
            />
          ) : (
            <FileOutlined style={{ fontSize: 24, color: '#1890ff' }} />
          )}
          <span
            style={{
              cursor: record.isDirectory ? 'pointer' : 'default',
              color: record.isDirectory ? '#1890ff' : 'inherit',
            }}
            onClick={() => record.isDirectory && handleEnterDirectory(record)}
          >
            {text}
          </span>
        </Space>
      ),
    },
    {
      title: '大小',
      dataIndex: 'size',
      key: 'size',
      width: 120,
      render: (size, record) =>
        record.isDirectory ? <span>-</span> : <Tag color="blue">{formatFileSize(size)}</Tag>,
    },
    {
      title: '修改时间',
      dataIndex: 'lastModified',
      key: 'lastModified',
      width: 180,
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_, record) => (
        <Space size="small">
          {!record.isDirectory && (
            <>
              <Tooltip title="预览">
                <Button
                  type="link"
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => handlePreview(record)}
                />
              </Tooltip>
              <Tooltip title="复制链接">
                <Button
                  type="link"
                  size="small"
                  icon={<LinkOutlined />}
                  onClick={() => copyLink(record)}
                />
              </Tooltip>
            </>
          )}
          <Popconfirm
            title="确认删除"
            description={
              record.isDirectory ? '确定要删除这个目录及其所有内容吗？' : '确定要删除这个文件吗？'
            }
            onConfirm={() => handleDelete(record.key)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="对象存储管理" subTitle="管理上传到对象存储的文件资源">
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="总文件数" value={files.length} prefix={<FileOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="总大小"
              value={formatFileSize(files.reduce((sum, f) => sum + f.size, 0))}
              prefix={<CloudUploadOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="已选择" value={selectedKeys.length} />
          </Card>
        </Col>
      </Row>

      <Card style={{ marginBottom: 16 }}>
        <Space wrap style={{ width: '100%' }} size={[8, 8]}>
          <Breadcrumb items={breadcrumbItems} />
          {currentPrefix && (
            <Button icon={<FolderOutlined />} onClick={handleGoBack}>
              返回上级目录
            </Button>
          )}
          {/* 调试信息：显示当前路径 */}
          {currentPrefix && <Tag color="blue">当前目录: {currentPrefix || '(根)'}</Tag>}
        </Space>
      </Card>

      <Card>
        <Space wrap style={{ marginBottom: 16, width: '100%' }} size={[8, 8]}>
          <Input
            placeholder="搜索文件名"
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            style={{ width: 'min(250px, 100%)' }}
            allowClear
          />
          <Button
            type="primary"
            icon={<UploadOutlined />}
            onClick={() => {
              setFileUploadList([]);
              setDirUploadList([]);
              setUploadStats({
                total: 0,
                success: 0,
                failed: 0,
                failedFiles: [],
              });
              setUploadVisible(true);
            }}
          >
            上传文件
          </Button>
          <Button icon={<FolderAddOutlined />} onClick={() => setCreateDirVisible(true)}>
            新建目录
          </Button>
          {selectedKeys.length > 0 && (
            <Popconfirm
              title="批量删除"
              description={`确定要删除选中的 ${selectedKeys.length} 个文件吗？`}
              onConfirm={handleBatchDelete}
              okText="确定"
              cancelText="取消"
            >
              <Button danger icon={<DeleteOutlined />}>
                批量删除
              </Button>
            </Popconfirm>
          )}
          <Button icon={<DownloadOutlined />} onClick={loadFiles}>
            刷新
          </Button>
        </Space>

        <Table
          columns={columns}
          dataSource={filteredFiles}
          rowKey="key"
          loading={loading}
          scroll={{ x: 960 }}
          rowSelection={{
            selectedRowKeys: selectedKeys,
            onChange: (keys) => setSelectedKeys(keys as string[]),
          }}
          pagination={{
            total: filteredFiles.length,
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 个文件`,
          }}
        />
      </Card>

      <Modal
        title="上传文件"
        open={uploadVisible}
        onCancel={() => {
          setUploadVisible(false);
          setFileUploadList([]);
          setDirUploadList([]);
          setUploadStats({
            total: 0,
            success: 0,
            failed: 0,
            failedFiles: [],
          });
          loadFiles(); // 关闭时刷新列表
        }}
        footer={
          <Button
            onClick={() => {
              setUploadVisible(false);
              setFileUploadList([]);
              setDirUploadList([]);
              setUploadStats({
                total: 0,
                success: 0,
                failed: 0,
                failedFiles: [],
              });
              loadFiles(); // 关闭时刷新列表
            }}
          >
            完成
          </Button>
        }
        width="min(700px, calc(100vw - 24px))"
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          {/* 上传进度统计 */}
          {(fileUploadList.length > 0 || dirUploadList.length > 0) && (
            <div style={{ padding: '12px', background: '#f5f5f5', borderRadius: '4px' }}>
              <Space direction="vertical" style={{ width: '100%' }} size="small">
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Text>总文件数: {fileUploadList.length + dirUploadList.length}</Text>
                  <Text>
                    已完成:{' '}
                    {fileUploadList.filter((f) => f.status === 'done').length +
                      dirUploadList.filter((f) => f.status === 'done').length}{' '}
                    / {fileUploadList.length + dirUploadList.length}
                  </Text>
                  {uploadStats.failed > 0 && <Text type="danger">失败: {uploadStats.failed}</Text>}
                </div>
                <Progress
                  percent={
                    Math.round(
                      ((fileUploadList.filter((f) => f.status === 'done').length +
                        dirUploadList.filter((f) => f.status === 'done').length) /
                        (fileUploadList.length + dirUploadList.length)) *
                        100,
                    ) || 0
                  }
                  status={uploadStats.failed > 0 ? 'exception' : 'active'}
                />
              </Space>
            </div>
          )}

          {/* 文件上传 */}
          <div>
            <div style={{ marginBottom: 8 }}>
              <Text strong>上传文件</Text>
            </div>
            <Upload.Dragger
              customRequest={handleUpload}
              multiple
              fileList={fileUploadList}
              onChange={(info) => {
                setFileUploadList(info.fileList);
              }}
              onRemove={(file) => {
                const index = fileUploadList.indexOf(file);
                const newFileList = fileUploadList.slice();
                newFileList.splice(index, 1);
                setFileUploadList(newFileList);
              }}
            >
              <p className="ant-upload-drag-icon">
                <CloudUploadOutlined style={{ fontSize: 48 }} />
              </p>
              <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
              <p className="ant-upload-hint">
                支持单个或批量上传。严禁上传公司数据或其他敏感文件。
              </p>
            </Upload.Dragger>
          </div>

          {/* 文件夹上传 */}
          <div>
            <div style={{ marginBottom: 8 }}>
              <Text strong>上传文件夹</Text>
            </div>
            <Upload.Dragger
              customRequest={handleUpload}
              multiple
              directory
              fileList={dirUploadList}
              onChange={(info) => {
                setDirUploadList(info.fileList);
              }}
              onRemove={(file) => {
                const index = dirUploadList.indexOf(file);
                const newFileList = dirUploadList.slice();
                newFileList.splice(index, 1);
                setDirUploadList(newFileList);
              }}
            >
              <p className="ant-upload-drag-icon">
                <FolderOutlined style={{ fontSize: 48 }} />
              </p>
              <p className="ant-upload-text">点击或拖拽文件夹到此区域上传</p>
              <p className="ant-upload-hint">支持整个文件夹上传，将保持原有的文件夹结构。</p>
            </Upload.Dragger>
          </div>
        </Space>
      </Modal>

      <Modal
        open={createDirVisible}
        title="新建目录"
        onOk={handleCreateDirectory}
        onCancel={() => {
          setCreateDirVisible(false);
          setDirName('');
        }}
        okText="创建"
        cancelText="取消"
      >
        <Input
          placeholder="请输入目录名称"
          value={dirName}
          onChange={(e) => setDirName(e.target.value)}
          prefix={<FolderOutlined />}
        />
      </Modal>

      <Modal
        open={previewVisible}
        title={previewFile.name}
        footer={null}
        onCancel={() => setPreviewVisible(false)}
        width="min(800px, calc(100vw - 24px))"
      >
        {isImageFile(previewFile.name) ? (
          <img alt={previewFile.name} style={{ width: '100%' }} src={previewFile.url} />
        ) : (
          <div style={{ padding: 40, textAlign: 'center' }}>
            <FileOutlined style={{ fontSize: 64, marginBottom: 16 }} />
            <p>此文件类型不支持预览</p>
            <Button
              type="primary"
              icon={<DownloadOutlined />}
              onClick={() => window.open(previewFile.url)}
            >
              下载文件
            </Button>
          </div>
        )}
      </Modal>
    </PageContainer>
  );
}
