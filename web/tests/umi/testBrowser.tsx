import React, { useEffect } from 'react';
import Login from '@/pages/User/Login';

type TestBrowserProps = {
  historyRef?: React.RefObject<{ push: (path: string) => void }>;
  location?: {
    pathname?: string;
  };
};

export const TestBrowser: React.FC<TestBrowserProps> = ({ historyRef }) => {
  useEffect(() => {
    if (historyRef) {
      historyRef.current = {
        push: () => {},
      };
    }
  }, [historyRef]);

  return <Login />;
};
