/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { BaseResponse } from './BaseResponse';
import type { Release } from './Release';
export type ReleaseListResponse = BaseResponse & {
  data?: {
    items?: Array<Release>;
  };
};
