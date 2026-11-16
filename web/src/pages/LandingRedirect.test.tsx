/** LandingRedirect（GUX-002/GUX-003 根路径按能力落地）覆盖。
 *
 * 权限优先级：运营（/console/home）→ 开发（/functions/catalog）→
 * 平台（/system）→ 全不可见回退个人中心（/account）。
 */
import { render } from '@testing-library/react';
import { history, useAccess } from '@umijs/max';
import LandingRedirect from './LandingRedirect';

jest.mock('@umijs/max', () => ({
  useAccess: jest.fn(),
  history: { push: jest.fn(), replace: jest.fn() },
}));

const mockedUseAccess = useAccess as unknown as jest.Mock<Record<string, boolean>, []>;
const mockedHistoryReplace = history.replace as unknown as jest.Mock;

beforeEach(() => {
  jest.clearAllMocks();
});

describe('LandingRedirect 落地规则', () => {
  it('运营能力命中运行控制台', () => {
    mockedUseAccess.mockReturnValue({ canConsoleRead: true, canFunctionsRead: true });
    render(<LandingRedirect />);
    expect(mockedHistoryReplace).toHaveBeenCalledWith('/console/home');
  });

  it('仅开发能力命中函数目录', () => {
    mockedUseAccess.mockReturnValue({ canFunctionsRead: true, canSystemConfigRead: true });
    render(<LandingRedirect />);
    expect(mockedHistoryReplace).toHaveBeenCalledWith('/functions/catalog');
  });

  it('仅平台能力命中系统管理', () => {
    mockedUseAccess.mockReturnValue({ canSystemConfigRead: true });
    render(<LandingRedirect />);
    expect(mockedHistoryReplace).toHaveBeenCalledWith('/system');
  });

  it('全部不可见回退个人中心', () => {
    mockedUseAccess.mockReturnValue({});
    render(<LandingRedirect />);
    expect(mockedHistoryReplace).toHaveBeenCalledWith('/account');
  });
});
