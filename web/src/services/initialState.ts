import { hydrateScope } from '@/stores/scope';
import { fetchServerFeatures, type ServerFeatures } from '@/services/api/features';

export type InitialCurrentUser = {
  name?: string;
  userid?: string;
  access?: string;
  roles?: string[];
  avatar?: string;
};

export type RuntimeInitialState = {
  currentUser?: InitialCurrentUser;
  features?: ServerFeatures;
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
  // Feature flags come from the public meta endpoint; fail-open inside.
  const features = await fetchServerFeatures();
  return {
    currentUser,
    features,
  };
}
