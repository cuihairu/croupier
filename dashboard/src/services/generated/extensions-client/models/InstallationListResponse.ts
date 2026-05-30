/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { BaseResponse } from './BaseResponse';
import type { Installation } from './Installation';
export type InstallationListResponse = BaseResponse & {
  data?: {
    items?: Array<Installation>;
    total?: number;
  };
};
