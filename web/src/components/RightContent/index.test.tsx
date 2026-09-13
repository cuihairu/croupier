/** RightContent 导出的两个轻量组件：SelectLang 包装透传 UmiSelectLang、
 * Question 渲染问号图标并 window.open 跳转文档。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { Question, SelectLang } from './index';

jest.mock('@umijs/max', () => {
  const React = require('react') as typeof import('react');
  return {
    __esModule: true,
    SelectLang: () => React.createElement('div', { 'data-testid': 'umi-select-lang' }),
  };
});

describe('RightContent', () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('SelectLang 包装渲染 UmiSelectLang', () => {
    render(<SelectLang />);
    expect(screen.getByTestId('umi-select-lang')).toBeInTheDocument();
  });

  it('Question 渲染问号图标，点击 window.open 打开文档地址', () => {
    const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
    const { container } = render(<Question />);
    expect(document.querySelector('.anticon-question-circle')).toBeInTheDocument();
    fireEvent.click(container.firstChild as HTMLElement);
    expect(openSpy).toHaveBeenCalledWith('https://pro.ant.design/docs/getting-started');
  });
});
