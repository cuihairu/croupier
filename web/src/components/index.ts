/**
 * 这个文件作为组件的目录
 * 目的是统一管理对外输出的组件，方便分类
 */
/**
 * 布局组件
 */
import Footer from './Footer';
import { Question, SelectLang } from './RightContent';
import { AvatarDropdown, AvatarName } from './RightContent/AvatarDropdown';
import {
  SummaryOverview,
  StandardFilterBar,
  StandardListSection,
  PageStatePanel,
  DASHBOARD_PAGE_TOKENS,
} from './StandardPage';

/**
 * Dashboard vNext 组件
 */
import PageRenderer from './PageRenderer';
import ResourcePageRenderer from './PageRenderer/ResourcePageRenderer';
import OperationPageRenderer from './PageRenderer/OperationPageRenderer';
import TaskPageRenderer from './PageRenderer/TaskPageRenderer';
import ReportPageRenderer from './PageRenderer/ReportPageRenderer';
import SchemaFormRenderer from './SchemaFormRenderer';

export {
  Footer,
  Question,
  SelectLang,
  AvatarDropdown,
  AvatarName,
  StandardFilterBar,
  SummaryOverview,
  StandardListSection,
  PageStatePanel,
  DASHBOARD_PAGE_TOKENS,
  // Dashboard vNext
  PageRenderer,
  ResourcePageRenderer,
  OperationPageRenderer,
  TaskPageRenderer,
  ReportPageRenderer,
  SchemaFormRenderer,
};
