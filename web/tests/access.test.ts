import access from '@/access';

describe('runtime access', () => {
  it('识别后端 profile/permissions 返回的管理员权限码', () => {
    const result = access({
      currentUser: {
        access: 'admin:all,*',
      },
    });

    expect(result.canAdmin).toBe(true);
    expect(result.canFunctionsRead).toBe(true);
    expect(result.canWorkspaceManage).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('识别 currentUser.roles 中的管理员角色', () => {
    const result = access({
      currentUser: {
        roles: ['super_admin'],
      },
    });

    expect(result.canAdmin).toBe(true);
    expect(result.canFunctionsRead).toBe(true);
    expect(result.canWorkspaceRead).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('识别当前后端使用的 workspace 单数权限码', () => {
    const result = access({
      currentUser: {
        access: 'functions:read,workspace:read,workspace:edit,workspace:publish',
      },
    });

    expect(result.canFunctionsRead).toBe(true);
    expect(result.canWorkspaceRead).toBe(true);
    expect(result.canWorkspaceEdit).toBe(true);
    expect(result.canWorkspacePublish).toBe(true);
    expect(result.canWorkspaceManage).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('允许只有函数调用权限的运行角色进入控制台', () => {
    const result = access({
      currentUser: {
        access: 'function:invoke',
      },
    });

    expect(result.canFunctionsRead).toBe(false);
    expect(result.canWorkspaceRead).toBe(false);
    expect(result.canConsoleRead).toBe(true);
  });
});
