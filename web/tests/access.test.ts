import access from '@/access';

describe('runtime access', () => {
  it('识别后端 profile/permissions 返回的管理员权限码', () => {
    const result = access({
      currentUser: {
        access: 'admin:all,*',
      },
    });

    expect(result.canAdmin).toBe(true);
    expect(result.canFunctionsAndPagesRead).toBe(true);
    expect(result.canFunctionsRead).toBe(true);
    expect(result.canPageManage).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('识别 currentUser.roles 中的管理员角色', () => {
    const result = access({
      currentUser: {
        roles: ['super_admin'],
      },
    });

    expect(result.canAdmin).toBe(true);
    expect(result.canFunctionsAndPagesRead).toBe(true);
    expect(result.canFunctionsRead).toBe(true);
    expect(result.canPageRead).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('识别 PageSpec 页面管理权限码', () => {
    const result = access({
      currentUser: {
        access: 'functions:read,pages:read,pages:edit,pages:publish',
      },
    });

    expect(result.canFunctionsRead).toBe(true);
    expect(result.canFunctionsAndPagesRead).toBe(true);
    expect(result.canPageRead).toBe(true);
    expect(result.canPageEdit).toBe(true);
    expect(result.canPagePublish).toBe(true);
    expect(result.canPageManage).toBe(true);
    expect(result.canConsoleRead).toBe(true);
  });

  it('允许只有函数调用权限的运行角色进入控制台', () => {
    const result = access({
      currentUser: {
        access: 'function:invoke',
      },
    });

    expect(result.canFunctionsRead).toBe(false);
    expect(result.canFunctionsAndPagesRead).toBe(false);
    expect(result.canPageRead).toBe(false);
    expect(result.canConsoleRead).toBe(true);
  });

  it('函数目录读取权限不等于运行控制台权限', () => {
    const result = access({
      currentUser: {
        access: 'functions:read',
      },
    });

    expect(result.canFunctionsRead).toBe(true);
    expect(result.canFunctionsAndPagesRead).toBe(true);
    expect(result.canPageRead).toBe(false);
    expect(result.canConsoleRead).toBe(false);
  });

  it('允许只有 PageSpec 读取权限的用户进入函数与页面入口', () => {
    const result = access({
      currentUser: {
        access: 'pages:read',
      },
    });

    expect(result.canFunctionsRead).toBe(false);
    expect(result.canPageRead).toBe(true);
    expect(result.canFunctionsAndPagesRead).toBe(true);
  });

  it('允许只有 OpenAPI Source 读取权限的用户进入函数与页面入口', () => {
    const result = access({
      currentUser: {
        access: 'openapi_sources:read',
      },
    });

    expect(result.canOpenAPISourcesRead).toBe(true);
    expect(result.canFunctionsRead).toBe(false);
    expect(result.canPageRead).toBe(false);
    expect(result.canResourcesRead).toBe(false);
    expect(result.canFunctionsAndPagesRead).toBe(true);
  });
});
