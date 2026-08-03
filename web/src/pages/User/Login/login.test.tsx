import { render, fireEvent, act } from '@testing-library/react';
import React, { useRef } from 'react';
import { TestBrowser } from '@@/testBrowser';
import { BRAND } from '@/config/branding';
import { history } from '@umijs/max';
import type { Location } from 'history';
import type { MemoryHistory } from 'history';

// @ts-ignore
import { startMock } from '@@/requestRecordMock';

declare const global: {
  __UMI_SET_INITIAL_STATE__?: jest.Mock;
};

const waitTime = (time: number = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

interface MockServer {
  close: () => void;
}

let server: MockServer;

function TestComponent({ onHistoryRef }: { onHistoryRef: (ref: React.MutableRefObject<MemoryHistory | undefined>) => void }) {
  const historyRef = useRef<MemoryHistory>();
  React.useEffect(() => {
    onHistoryRef(historyRef);
  }, []);
  return (
    <TestBrowser
      historyRef={historyRef as unknown as React.MutableRefObject<Location>}
      location={{
        pathname: '/user/login',
      }}
    />
  );
}

describe('Login Page', () => {
  beforeAll(async () => {
    server = await startMock({
      port: 8000,
      scene: 'login',
    });
  });

  afterAll(() => {
    server?.close();
  });

  it('should show login form', async () => {
    let historyRef: React.MutableRefObject<MemoryHistory | undefined>;
    const rootContainer = render(
      <TestComponent onHistoryRef={(ref) => { historyRef = ref; }} />
    );

    await rootContainer.findAllByText(BRAND.title);

    await act(async () => {
      await waitTime(100);
      historyRef.current?.push('/user/login');
    });

    expect(rootContainer.baseElement?.querySelector('.ant-pro-form-login-desc')?.textContent).toBe(
      BRAND.subTitle,
    );

    rootContainer.unmount();
  });

  it('should login success', async () => {
    let historyRef: React.MutableRefObject<MemoryHistory | undefined>;
    const rootContainer = render(
      <TestComponent onHistoryRef={(ref) => { historyRef = ref; }} />
    );

    await rootContainer.findAllByText(BRAND.title);

    const userNameInput = await rootContainer.findByPlaceholderText('用户名: admin or user');

    act(() => {
      fireEvent.change(userNameInput, { target: { value: 'admin' } });
    });

    const passwordInput = await rootContainer.findByPlaceholderText('密码: admin');

    act(() => {
      fireEvent.change(passwordInput, { target: { value: 'ant.design' } });
    });

    const submitButton = await rootContainer.findByRole('button', { name: /登\s*录/ });
    await submitButton.click();

    await waitTime(200);

    expect(localStorage.setItem).toHaveBeenCalledWith('token', 'test-token');
    expect(global.__UMI_SET_INITIAL_STATE__).toHaveBeenCalled();
    expect(history.push).toHaveBeenCalledWith('/');

    rootContainer.unmount();
  });
});
