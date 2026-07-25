import { hydrateScope } from '@/stores/scope';

export type InitialCurrentUser = {
  name?: string;
  userid?: string;
  access?: string;
  roles?: string[];
  avatar?: string;
};

export type RuntimeInitialState = {
  currentUser?: InitialCurrentUser;
};

export async function loadAuthedInitialState(
  fetchUserInfo: () => Promise<InitialCurrentUser | undefined>,
): Promise<RuntimeInitialState> {
  const currentUser = await fetchUserInfo();
  if (!currentUser) {
    return {
      currentUser: undefined,
    };
  }
  hydrateScope();
  return {
    currentUser,
  };
}
