/**
 * 函数推荐引擎
 *
 * 根据当前 Tab 已有函数推荐相关函数。
 * 推荐依据：
 * - 同实体函数
 * - 函数调用关系（上游/下游）
 * - 相同操作类型
 * - 相同标签
 *
 * @module pages/WorkspaceEditor/utils/functionRecommender
 */

import type { FunctionDescriptor } from '@/services/api/functions';

/** 推荐原因 */
export type RecommendationReason =
  | 'same-entity' // 同实体
  | 'same-operation' // 同操作类型
  | 'related-entity' // 相关实体（如 player -> player_bag）
  | 'input-output-match' // 输入输出匹配
  | 'common-pair' // 常用组合
  | 'tag-match'; // 标签匹配

/** 推荐结果 */
export interface FunctionRecommendation {
  function: FunctionDescriptor;
  reason: RecommendationReason;
  score: number; // 相关性分数 0-100
  description: string; // 推荐理由说明
}

/** 推荐选项 */
export interface RecommendationOptions {
  /** 最大推荐数量 */
  maxResults?: number;
  /** 排除已添加的函数 */
  excludeExisting?: boolean;
  /** 最小相关性分数 */
  minScore?: number;
}

/**
 * 根据已有函数列表推荐相关函数
 */
export function recommendFunctions(
  existingFunctions: string[],
  allDescriptors: FunctionDescriptor[],
  options: RecommendationOptions = {},
): FunctionRecommendation[] {
  const { maxResults = 6, excludeExisting = true, minScore = 20 } = options;

  // 获取已有函数的描述符
  const existingDescriptors = allDescriptors.filter((d) => existingFunctions.includes(d.id));

  if (existingDescriptors.length === 0) {
    // 如果没有已有函数，返回常用函数
    return getCommonFunctions(allDescriptors, maxResults);
  }

  // 收集所有推荐
  const recommendations = new Map<string, FunctionRecommendation>();

  // 遍历所有未添加的函数
  allDescriptors.forEach((descriptor) => {
    // 排除已添加的函数
    if (excludeExisting && existingFunctions.includes(descriptor.id)) {
      return;
    }

    // 计算推荐分数和原因
    const results = analyzeRecommendation(descriptor, existingDescriptors, allDescriptors);

    // 取最高分的原因
    const bestResult = results.sort((a, b) => b.score - a.score)[0];

    if (bestResult && bestResult.score >= minScore) {
      // 如果同一函数已有推荐，只保留分数更高的
      const existing = recommendations.get(descriptor.id);
      if (!existing || existing.score < bestResult.score) {
        recommendations.set(descriptor.id, bestResult);
      }
    }
  });

  // 按分数排序并返回前 N 个
  return Array.from(recommendations.values())
    .sort((a, b) => b.score - a.score)
    .slice(0, maxResults);
}

/**
 * 分析单个函数的推荐理由
 */
function analyzeRecommendation(
  descriptor: FunctionDescriptor,
  existingDescriptors: FunctionDescriptor[],
  allDescriptors: FunctionDescriptor[],
): FunctionRecommendation[] {
  const results: FunctionRecommendation[] = [];

  for (const existing of existingDescriptors) {
    // 1. 同实体推荐
    if (descriptor.entity && existing.entity && descriptor.entity === existing.entity) {
      if (descriptor.operation !== existing.operation) {
        results.push({
          function: descriptor,
          reason: 'same-entity',
          score: 70,
          description: `与 ${existing.displayName?.zh || existing.id} 同属 ${
            existing.entityDisplay?.zh || existing.entity
          } 实体`,
        });
      }
    }

    // 2. 相关实体推荐（如 player 和 player_bag）
    if (descriptor.entity && existing.entity) {
      if (isRelatedEntity(descriptor.entity, existing.entity)) {
        results.push({
          function: descriptor,
          reason: 'related-entity',
          score: 60,
          description: `实体 ${descriptor.entityDisplay?.zh || descriptor.entity} 与 ${
            existing.entityDisplay?.zh || existing.entity
          } 相关`,
        });
      }
    }

    // 3. 同操作类型推荐
    if (descriptor.operation && existing.operation && descriptor.operation === existing.operation) {
      if (descriptor.entity !== existing.entity) {
        results.push({
          function: descriptor,
          reason: 'same-operation',
          score: 40,
          description: `${existing.operationDisplay?.zh || existing.operation} 类函数`,
        });
      }
    }

    // 4. 输入输出匹配
    const inputMatch = checkInputOutputMatch(descriptor, existing);
    if (inputMatch) {
      results.push({
        function: descriptor,
        reason: 'input-output-match',
        score: 80,
        description: inputMatch,
      });
    }

    // 5. 常用组合
    const pairScore = getCommonPairScore(descriptor.id, existing.id);
    if (pairScore > 0) {
      results.push({
        function: descriptor,
        reason: 'common-pair',
        score: 50 + pairScore,
        description: `常与 ${existing.displayName?.zh || existing.id} 一起使用`,
      });
    }

    // 6. 标签匹配
    if (descriptor.tags && existing.tags) {
      const commonTags = descriptor.tags.filter((t) => existing.tags?.includes(t));
      if (commonTags.length >= 2) {
        results.push({
          function: descriptor,
          reason: 'tag-match',
          score: 30 + commonTags.length * 10,
          description: `共享标签: ${commonTags.slice(0, 2).join(', ')}`,
        });
      }
    }
  }

  return results;
}

/**
 * 检查两个实体是否相关
 */
function isRelatedEntity(entity1: string, entity2: string): boolean {
  // 移除后缀后比较
  const base1 = entity1.replace(/_(bag|mail|log|history|stat)$/i, '');
  const base2 = entity2.replace(/_(bag|mail|log|history|stat)$/i, '');
  return base1 === base2 && entity1 !== entity2;
}

/**
 * 检查输入输出是否匹配
 */
function checkInputOutputMatch(
  descriptor: FunctionDescriptor,
  existing: FunctionDescriptor,
): string | null {
  try {
    const inputSchema = descriptor.inputSchema
      ? JSON.parse(descriptor.inputSchema)
      : descriptor.params;
    const outputSchema = existing.outputSchema
      ? JSON.parse(existing.outputSchema)
      : existing.outputs;

    if (!inputSchema?.properties || !outputSchema?.properties) {
      return null;
    }

    const inputProps = Object.keys(inputSchema.properties);
    const outputProps = Object.keys(outputSchema.properties);

    // 计算匹配度
    const matches = inputProps.filter((p) => outputProps.includes(p));
    const matchRatio = matches.length / Math.max(inputProps.length, 1);

    if (matches.length >= 2 && matchRatio >= 0.3) {
      return `可使用 ${existing.displayName?.zh || existing.id} 的输出作为输入 (${
        matches.length
      } 个字段匹配)`;
    }

    return null;
  } catch {
    return null;
  }
}

/**
 * 获取常用组合分数（模拟数据，实际应从使用统计获取）
 */
function getCommonPairScore(func1: string, func2: string): number {
  // 这里是示例数据，实际应该从日志/统计中获取
  const commonPairs: Record<string, Record<string, number>> = {
    'player.query': { 'player_bag.list': 30, 'player_mail.list': 25 },
    'order.query': { 'order_detail.list': 35, 'order_item.list': 30 },
    'player.create': { 'player_bag.add': 40 },
    'player.update': { 'player_bag.update': 35 },
  };

  return commonPairs[func1]?.[func2] || commonPairs[func2]?.[func1] || 0;
}

/**
 * 获取常用函数（当没有已有函数时）
 */
function getCommonFunctions(
  allDescriptors: FunctionDescriptor[],
  maxResults: number,
): FunctionRecommendation[] {
  // 常用操作优先级
  const operationPriority: Record<string, number> = {
    list: 100,
    query: 90,
    create: 70,
    update: 60,
    delete: 50,
  };

  return allDescriptors
    .filter((d) => d.operation && operationPriority[d.operation])
    .sort((a, b) => {
      const scoreA = operationPriority[a.operation || ''] || 0;
      const scoreB = operationPriority[b.operation || ''] || 0;
      return scoreB - scoreA;
    })
    .slice(0, maxResults)
    .map((d) => ({
      function: d,
      reason: 'same-operation' as const,
      score: operationPriority[d.operation || ''] || 50,
      description: `常用 ${d.operationDisplay?.zh || d.operation} 操作`,
    }));
}

/**
 * 获取推荐理由的显示文本
 */
export function getReasonText(reason: RecommendationReason): string {
  const reasonTexts: Record<RecommendationReason, string> = {
    'same-entity': '同实体',
    'same-operation': '同操作',
    'related-entity': '相关实体',
    'input-output-match': '数据流匹配',
    'common-pair': '常用组合',
    'tag-match': '标签匹配',
  };
  return reasonTexts[reason] || '相关推荐';
}

/**
 * 获取推荐理由的颜色
 */
export function getReasonColor(reason: RecommendationReason): string {
  const colors: Record<RecommendationReason, string> = {
    'same-entity': 'blue',
    'same-operation': 'cyan',
    'related-entity': 'purple',
    'input-output-match': 'green',
    'common-pair': 'orange',
    'tag-match': 'geekblue',
  };
  return colors[reason] || 'default';
}
