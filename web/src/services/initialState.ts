import type { WorkspaceConfig } from '@/types/workspace';
import { hydrateScope } from '@/stores/scope';
import { listPublishedWorkspaceConfigs } from '@/services/workspaceConfig';

export type InitialCurrentUser = {
  name?: string;
  userid?: string;
  access?: string;
  roles?: string[];
  avatar?: string;
};

export type RuntimeInitialState = {
  currentUser?: InitialCurrentUser;
  workspaceConfigs?: WorkspaceConfig[];
};

export async function loadConsoleWorkspaceConfigs(): Promise<WorkspaceConfig[]> {
  return listPublishedWorkspaceConfigs({ skipErrorHandler: true });
}

export async function loadAuthedInitialState(
  fetchUserInfo: () => Promise<InitialCurrentUser | undefined>,
): Promise<RuntimeInitialState> {
  const currentUser = await fetchUserInfo();
  if (!currentUser) {
    return {
      currentUser: undefined,
      workspaceConfigs: [],
    };
  }
  hydrateScope();
  return {
    currentUser,
    workspaceConfigs: await loadConsoleWorkspaceConfigs(),
  };
}
