import { Request, Response } from 'express';

// GamesManage 域的静态 mock（games 列表 / 详情 / 编辑保存 / 图标上传）。
// 图标 key 与真实上传端点同形：内容寻址 icons/games/<16hex>.<ext>，URL 即
// /uploads/<key>（file 驱动未配 PublicURL 时的相对地址口径）。

const games = [
  {
    id: 1,
    gameId: 'demo_game',
    name: 'demo_game',
    aliasName: '示例游戏',
    icon: '/uploads/icons/games/0123456789abcdef.png',
    status: 'online',
    enabled: true,
    envs: ['prod', 'dev'],
  },
  {
    id: 2,
    gameId: 'croupier_demo',
    name: 'croupier_demo',
    aliasName: 'Croupier Demo',
    icon: '',
    status: 'online',
    enabled: true,
    envs: ['prod'],
  },
];

export default {
  'GET /api/v1/games': (req: Request, res: Response) => {
    res.json({ games });
  },
  'GET /api/v1/games/:id': (req: Request, res: Response) => {
    const game = games.find((item) => String(item.id) === String(req.params.id));
    if (!game) {
      res.status(404).json({ error: 'not_found', message: '游戏不存在' });
      return;
    }
    res.json({ game });
  },
  'PUT /api/v1/games/:id': (req: Request, res: Response) => {
    const game = games.find((item) => String(item.id) === String(req.params.id));
    if (!game) {
      res.status(404).json({ error: 'not_found', message: '游戏不存在' });
      return;
    }
    const body = (req.body || {}) as { aliasName?: string; icon?: string };
    if (body.aliasName !== undefined) game.aliasName = body.aliasName;
    if (body.icon !== undefined) game.icon = body.icon;
    res.json({ game });
  },
  'POST /api/v1/games/icons': (req: Request, res: Response) => {
    res.json({
      key: 'icons/games/0123456789abcdef.png',
      url: '/uploads/icons/games/0123456789abcdef.png',
    });
  },
};
