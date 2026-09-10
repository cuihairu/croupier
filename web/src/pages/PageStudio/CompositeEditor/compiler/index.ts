/** 编译器门面：树 ↔ CompositeSection 双向变换的公开出口。
 * 实现拆分在 types（共享类型/常量）、normalize（参数归一）、
 * compile（树→sections）、decompile（sections→树）四模块；
 * 对外导出面保持拆分前的单文件形态（调用方与测试导入路径零改动）。 */
export { compileTree } from './compile';
export { decompileToTree } from './decompile';
export { SECTION_KEY_RE } from './types';
export type { CompiledSection, CompiledAction, CompileResult, SpecSectionLike } from './types';
