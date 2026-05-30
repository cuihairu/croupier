/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { BaseResponse } from '../models/BaseResponse';
import type { CapabilitiesResponse } from '../models/CapabilitiesResponse';
import type { CatalogDetailResponse } from '../models/CatalogDetailResponse';
import type { CatalogListResponse } from '../models/CatalogListResponse';
import type { EventListResponse } from '../models/EventListResponse';
import type { InstallationDetailResponse } from '../models/InstallationDetailResponse';
import type { InstallationListResponse } from '../models/InstallationListResponse';
import type { InstallRequest } from '../models/InstallRequest';
import type { ReleaseListResponse } from '../models/ReleaseListResponse';
import type { CancelablePromise } from '../core/CancelablePromise';
import { OpenAPI } from '../core/OpenAPI';
import { request as __request } from '../core/request';
export class DefaultService {
  /**
   * List extension catalog
   * @param page
   * @param pageSize
   * @returns CatalogListResponse OK
   * @throws ApiError
   */
  public static getExtensionsCatalog(
    page: number = 1,
    pageSize: number = 20,
  ): CancelablePromise<CatalogListResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/catalog',
      query: {
        page: page,
        page_size: pageSize,
      },
    });
  }
  /**
   * Get catalog detail
   * @param id
   * @returns CatalogDetailResponse OK
   * @throws ApiError
   */
  public static getExtensionsCatalog1(id: string): CancelablePromise<CatalogDetailResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/catalog/{id}',
      path: {
        id: id,
      },
    });
  }
  /**
   * List catalog releases
   * @param id
   * @returns ReleaseListResponse OK
   * @throws ApiError
   */
  public static getExtensionsCatalogReleases(id: string): CancelablePromise<ReleaseListResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/catalog/{id}/releases',
      path: {
        id: id,
      },
    });
  }
  /**
   * List extension installations
   * @param page
   * @param pageSize
   * @returns InstallationListResponse OK
   * @throws ApiError
   */
  public static getExtensionsInstallations(
    page: number = 1,
    pageSize: number = 20,
  ): CancelablePromise<InstallationListResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/installations',
      query: {
        page: page,
        page_size: pageSize,
      },
    });
  }
  /**
   * Get installation detail
   * @param id
   * @returns InstallationDetailResponse OK
   * @throws ApiError
   */
  public static getExtensionsInstallations1(
    id: string,
  ): CancelablePromise<InstallationDetailResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/installations/{id}',
      path: {
        id: id,
      },
    });
  }
  /**
   * Install extension release
   * @param requestBody
   * @returns InstallationDetailResponse OK
   * @throws ApiError
   */
  public static postExtensionsInstall(
    requestBody: InstallRequest,
  ): CancelablePromise<InstallationDetailResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/install',
      body: requestBody,
      mediaType: 'application/json',
      errors: {
        409: `Conflict`,
      },
    });
  }
  /**
   * Enable extension installation
   * @param id
   * @returns InstallationDetailResponse OK
   * @throws ApiError
   */
  public static postExtensionsEnable(id: string): CancelablePromise<InstallationDetailResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/{id}/enable',
      path: {
        id: id,
      },
    });
  }
  /**
   * Disable extension installation
   * @param id
   * @returns InstallationDetailResponse OK
   * @throws ApiError
   */
  public static postExtensionsDisable(id: string): CancelablePromise<InstallationDetailResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/{id}/disable',
      path: {
        id: id,
      },
    });
  }
  /**
   * Upgrade extension installation
   * @param id
   * @param requestBody
   * @returns InstallationDetailResponse OK
   * @throws ApiError
   */
  public static postExtensionsUpgrade(
    id: string,
    requestBody: {
      target_version?: string;
    },
  ): CancelablePromise<InstallationDetailResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/{id}/upgrade',
      path: {
        id: id,
      },
      body: requestBody,
      mediaType: 'application/json',
    });
  }
  /**
   * Uninstall extension installation
   * @param id
   * @returns BaseResponse OK
   * @throws ApiError
   */
  public static deleteExtensionsUninstall(id: string): CancelablePromise<BaseResponse> {
    return __request(OpenAPI, {
      method: 'DELETE',
      url: '/extensions/{id}/uninstall',
      path: {
        id: id,
      },
    });
  }
  /**
   * Get extension capabilities
   * @param id
   * @returns CapabilitiesResponse OK
   * @throws ApiError
   */
  public static getExtensionsCapabilities(id: string): CancelablePromise<CapabilitiesResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/{id}/capabilities',
      path: {
        id: id,
      },
    });
  }
  /**
   * Trigger extension health check
   * @param id
   * @returns BaseResponse OK
   * @throws ApiError
   */
  public static postExtensionsHealthCheck(id: string): CancelablePromise<BaseResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/{id}/health-check',
      path: {
        id: id,
      },
    });
  }
  /**
   * Trigger extension reconcile
   * @param id
   * @returns BaseResponse OK
   * @throws ApiError
   */
  public static postExtensionsReconcile(id: string): CancelablePromise<BaseResponse> {
    return __request(OpenAPI, {
      method: 'POST',
      url: '/extensions/{id}/reconcile',
      path: {
        id: id,
      },
    });
  }
  /**
   * List extension events
   * @param id
   * @param page
   * @param pageSize
   * @returns EventListResponse OK
   * @throws ApiError
   */
  public static getExtensionsEvents(
    id: string,
    page: number = 1,
    pageSize: number = 20,
  ): CancelablePromise<EventListResponse> {
    return __request(OpenAPI, {
      method: 'GET',
      url: '/extensions/{id}/events',
      path: {
        id: id,
      },
      query: {
        page: page,
        page_size: pageSize,
      },
    });
  }
}
