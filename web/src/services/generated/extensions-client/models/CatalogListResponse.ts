/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { BaseResponse } from './BaseResponse';
import type { CatalogItem } from './CatalogItem';
export type CatalogListResponse = BaseResponse & {
  data?: {
    items?: Array<CatalogItem>;
    total?: number;
  };
};
