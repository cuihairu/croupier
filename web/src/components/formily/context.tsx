import React, { createContext, useContext } from 'react';

export interface FormilyRuntimeContext {
  gameId?: string;
  env?: string;
  functionId?: string;
  permissions?: string[];
  meta?: Record<string, any>;
}

const FormilyContext = createContext<FormilyRuntimeContext>({});

export function useFormilyContext() {
  return useContext(FormilyContext);
}

export function FormilyContextProvider({
  value,
  children,
}: {
  value: FormilyRuntimeContext;
  children: React.ReactNode;
}) {
  return <FormilyContext.Provider value={value}>{children}</FormilyContext.Provider>;
}
