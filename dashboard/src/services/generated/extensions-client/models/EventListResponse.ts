/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { BaseResponse } from './BaseResponse';
import type { ExtensionEvent } from './ExtensionEvent';
export type EventListResponse = BaseResponse & {
  data?: {
    items?: Array<ExtensionEvent>;
    total?: number;
  };
};
