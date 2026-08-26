import { GithubOutlined } from '@ant-design/icons';
import { DefaultFooter } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import React from 'react';

const defaultLinks = [
  {
    key: 'croupier',
    title: (
      <span>
        <GithubOutlined style={{ marginRight: 6 }} /> Croupier
      </span>
    ),
    href: 'https://github.com/cuihairu/croupier',
    blankTarget: true,
  },
];

const Footer: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const cfg = initialState?.siteConfig;
  const links = (
    cfg?.footerLinks && cfg.footerLinks.length > 0
      ? cfg.footerLinks.map((l) => ({ key: l.key, title: l.title, href: l.url, blankTarget: true }))
      : defaultLinks
  ) as { key: string; title: React.ReactNode; href: string; blankTarget: boolean }[];
  return (
    <DefaultFooter
      style={{
        background: 'none',
      }}
      copyright={cfg?.footerCopyright || false}
      links={links}
    />
  );
};

export default Footer;
