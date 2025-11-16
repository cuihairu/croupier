import React from 'react';
import Link from '@docusaurus/Link';

export default function Home() {
  return (
    <main style={{maxWidth: 960, margin: '0 auto', padding: '4rem 1rem'}}>
      <h1 style={{marginBottom: '0.5rem'}}>Croupier 文档</h1>
      <p style={{color: '#666', marginTop: 0}}>
        分布式游戏管理系统文档。侧边栏包含全部文档目录（自动生成）。
      </p>
      <div style={{display: 'flex', gap: '1rem', flexWrap: 'wrap', marginTop: '2rem'}}>
        <Link className="button button--primary" to="/ARCHITECTURE">
          架构总览
        </Link>
        <Link className="button button--secondary" to="/config">
          配置指南
        </Link>
        <Link className="button button--secondary" to="/CPP_SDK_DOCS_INDEX">
          C++ SDK
        </Link>
        <Link className="button button--secondary" to="/deployment">
          部署
        </Link>
        <Link className="button button--secondary" to="/README">
          所有文档
        </Link>
      </div>
    </main>
  );
}