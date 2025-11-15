import { createHotReloadClient, FunctionDescriptor, FunctionHandler } from '../src';

// 创建热重载配置
const config = {
  agentAddr: '127.0.0.1:19090',
  autoReconnect: true,
  reconnectDelay: 5000,
  fileWatching: {
    enabled: true,
    watchDir: './functions',
    patterns: ['*.ts', '*.js', '*.json']
  },
  tools: {
    nodemon: true,
    pm2: false,
    moduleReload: true
  }
};

// 创建客户端
const client = createHotReloadClient(config);

// 定义游戏函数
const playerBanHandler: FunctionHandler = async (context: string, payload: string): Promise<string> => {
  console.log(`🚫 Player ban requested: ${payload}`);

  try {
    const data = JSON.parse(payload);
    const playerId = data.player_id;
    const reason = data.reason || 'No reason provided';

    // 模拟玩家封禁逻辑
    await new Promise(resolve => setTimeout(resolve, 100));

    return JSON.stringify({
      status: 'success',
      player_id: playerId,
      action: 'banned',
      reason: reason,
      banned_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString() // 24小时后解封
    });
  } catch (error) {
    return JSON.stringify({
      status: 'error',
      message: 'Invalid payload format',
      error: error instanceof Error ? error.message : 'Unknown error'
    });
  }
};

const walletTransferHandler: FunctionHandler = async (context: string, payload: string): Promise<string> => {
  console.log(`💰 Wallet transfer requested: ${payload}`);

  try {
    const data = JSON.parse(payload);
    const { from_player_id, to_player_id, amount, currency = 'gold' } = data;

    // 模拟转账逻辑
    await new Promise(resolve => setTimeout(resolve, 200));

    return JSON.stringify({
      status: 'success',
      transaction_id: `tx_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      from_player_id,
      to_player_id,
      amount: parseFloat(amount),
      currency,
      fee: parseFloat(amount) * 0.02, // 2% 手续费
      net_amount: parseFloat(amount) * 0.98,
      processed_at: new Date().toISOString()
    });
  } catch (error) {
    return JSON.stringify({
      status: 'error',
      message: 'Transfer failed',
      error: error instanceof Error ? error.message : 'Unknown error'
    });
  }
};

const shopBuyHandler: FunctionHandler = async (context: string, payload: string): Promise<string> => {
  console.log(`🛒 Shop purchase requested: ${payload}`);

  try {
    const data = JSON.parse(payload);
    const { player_id, item_id, quantity = 1 } = data;

    // 模拟商店购买逻辑
    await new Promise(resolve => setTimeout(resolve, 150));

    const items = {
      'sword_001': { name: 'Iron Sword', price: 100 },
      'potion_001': { name: 'Health Potion', price: 50 },
      'armor_001': { name: 'Leather Armor', price: 200 }
    };

    const item = items[item_id as keyof typeof items];
    if (!item) {
      return JSON.stringify({
        status: 'error',
        message: 'Item not found',
        item_id
      });
    }

    const totalPrice = item.price * quantity;

    return JSON.stringify({
      status: 'success',
      purchase_id: `pur_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      player_id,
      item: {
        id: item_id,
        name: item.name,
        quantity,
        unit_price: item.price,
        total_price: totalPrice
      },
      purchased_at: new Date().toISOString()
    });
  } catch (error) {
    return JSON.stringify({
      status: 'error',
      message: 'Purchase failed',
      error: error instanceof Error ? error.message : 'Unknown error'
    });
  }
};

// 定义函数描述符
const playerBanDescriptor: FunctionDescriptor = {
  id: 'player.ban',
  version: '1.0.0',
  name: 'Ban Player',
  description: 'Ban a player from the game for a specified period',
  input_schema: {
    type: 'object',
    properties: {
      player_id: { type: 'string' },
      reason: { type: 'string' },
      duration_hours: { type: 'number', default: 24 }
    },
    required: ['player_id']
  }
};

const walletTransferDescriptor: FunctionDescriptor = {
  id: 'wallet.transfer',
  version: '1.0.0',
  name: 'Wallet Transfer',
  description: 'Transfer currency between player wallets',
  input_schema: {
    type: 'object',
    properties: {
      from_player_id: { type: 'string' },
      to_player_id: { type: 'string' },
      amount: { type: 'number' },
      currency: { type: 'string', default: 'gold' }
    },
    required: ['from_player_id', 'to_player_id', 'amount']
  }
};

const shopBuyDescriptor: FunctionDescriptor = {
  id: 'shop.buy',
  version: '1.0.0',
  name: 'Shop Purchase',
  description: 'Purchase items from the game shop',
  input_schema: {
    type: 'object',
    properties: {
      player_id: { type: 'string' },
      item_id: { type: 'string' },
      quantity: { type: 'number', default: 1 }
    },
    required: ['player_id', 'item_id']
  }
};

async function main(): Promise<void> {
  console.log('🚀 Starting Croupier TypeScript SDK Demo');
  console.log('=========================================');

  try {
    // 注册函数
    client
      .registerFunction(playerBanDescriptor, playerBanHandler)
      .registerFunction(walletTransferDescriptor, walletTransferHandler)
      .registerFunction(shopBuyDescriptor, shopBuyHandler);

    console.log('📝 Functions registered successfully');

    // 设置事件监听
    client.on('connected', () => {
      console.log('✅ Connected to Croupier Agent');
      console.log('🔥 Hot reload is active - modify function files to see live updates');
    });

    client.on('functionReloaded', (functionId: string, descriptor: FunctionDescriptor) => {
      console.log(`🔄 Function reloaded: ${functionId} (${descriptor.version})`);
    });

    client.on('moduleReloaded', (filePath: string) => {
      console.log(`📁 Module reloaded: ${filePath}`);
    });

    client.on('configFileChanged', (filePath: string) => {
      console.log(`⚙️ Config file changed: ${filePath}`);
    });

    client.on('reconnected', () => {
      console.log('🔄 Reconnected to Agent');
    });

    client.on('connectionError', (error: Error) => {
      console.error('❌ Connection error:', error.message);
    });

    // 连接到Agent
    await client.connect();

    // 定期输出状态
    const statusInterval = setInterval(() => {
      const status = client.getReloadStatus();
      console.log('\n📊 Reload Status:');
      console.log(`  Connection: ${status.connectionStatus}`);
      console.log(`  Functions: ${status.functionsCount}`);
      console.log(`  Reconnects: ${status.reconnectCount}`);
      console.log(`  Function reloads: ${status.functionReloads}`);
      console.log(`  Uptime: ${(status.uptime / 1000).toFixed(1)}s`);
    }, 30000);

    // 优雅关闭处理
    const gracefulShutdown = async (): Promise<void> => {
      console.log('\n🛑 Graceful shutdown initiated...');
      clearInterval(statusInterval);

      try {
        await client.gracefulShutdown(5000);
        console.log('✅ Shutdown completed');
        process.exit(0);
      } catch (error) {
        console.error('❌ Shutdown error:', error);
        process.exit(1);
      }
    };

    // 监听关闭信号
    process.on('SIGINT', gracefulShutdown);
    process.on('SIGTERM', gracefulShutdown);

    console.log('\n🎮 Demo running! Press Ctrl+C to exit');
    console.log('💡 Try modifying function files to see hot reload in action');

  } catch (error) {
    console.error('❌ Demo failed to start:', error);
    process.exit(1);
  }
}

// 启动演示
main().catch((error) => {
  console.error('💥 Unhandled error:', error);
  process.exit(1);
});

export default main;