/**
 * 页面生成器 - 入口组件
 *
 * 根据配置动态生成不同类型的页面
 */

import React from 'react';
import { Result } from 'antd';
import type { PageConfig } from './types';
import ListPage from './ListPage';
import FormPage from './FormPage';
import DetailPage from './DetailPage';

interface PageGeneratorProps {
  config: PageConfig;
}

const PageGenerator: React.FC<PageGeneratorProps> = ({ config }) => {
  // 根据页面类型渲染不同的页面
  switch (config.type) {
    case 'list':
      return <ListPage config={config} />;

    case 'form':
      return <FormPage config={config} />;

    case 'detail':
      return <DetailPage config={config} />;

    default:
      return (
        <Result
          status="warning"
          title="不支持的页面类型"
          subTitle={`页面类型 "${config.type}" 暂未实现`}
        />
      );
  }
};

export default PageGenerator;
