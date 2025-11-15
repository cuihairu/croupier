const HotReloadableClient = require('./src/hotreload-client');

// 基础配置
const config = {
  agentAddr: '127.0.0.1:19090',
  autoReconnect: true,
  reconnectDelay: 5000,
  maxRetryAttempts: 5,
  healthCheckInterval: 30000,
  gracefulShutdownTimeout: 30000,

  // 文件监听配置
  fileWatching: {
    enabled: true,
    watchDir: './functions',
    patterns: ['*.js', '*.json']
  },

  // 工具集成
  tools: {
    nodemon: true,      // Nodemon支持
    pm2: false,         // PM2支持
    moduleReload: true  // 模块缓存清除
  }
};

// 创建热重载客户端
const client = new HotReloadableClient(config);

// 游戏函数：玩家封禁
const playerBanDescriptor = {
  id: 'player.ban',
  version: '1.0.0'
};

function handlePlayerBan(payload) {
  console.log(`🚫 Processing player ban: ${JSON.stringify(payload)}`);

  // 模拟处理延迟
  return new Promise(resolve => {
    setTimeout(() => {
      resolve({
        result: 'success',
        message: 'Player banned',
        timestamp: new Date().toISOString()
      });
    }, 100);
  });
}

// 游戏函数：服务器状态
const serverStatusDescriptor = {
  id: 'server.status',
  version: '1.0.0'
};

function handleServerStatus(payload) {
  console.log(`📊 Processing server status request: ${JSON.stringify(payload)}`);

  return {
    status: 'running',
    uptime: process.uptime(),
    memory: {
      used: process.memoryUsage().rss / 1024 / 1024,
      heap: process.memoryUsage().heapUsed / 1024 / 1024
    },
    connections: 42,
    timestamp: new Date().toISOString()
  };
}

// 启动服务
async function main() {
  console.log('🔥 Starting Croupier Node.js Client with Hot Reload');

  try {
    // 注册函数
    client
      .registerFunction(playerBanDescriptor, handlePlayerBan)
      .registerFunction(serverStatusDescriptor, handleServerStatus);

    // 连接到Agent
    await client.connect();

    // 监听事件
    client.on('connected', () => {
      console.log('🎉 Client connected and ready');
    });

    client.on('functionReloaded', (functionId, descriptor) => {
      console.log(`🔄 Function reloaded: ${functionId} v${descriptor.version}`);
    });

    client.on('reconnected', () => {
      console.log('🔄 Client reconnected successfully');
    });

    client.on('connectionError', (error) => {
      console.error('❌ Connection error:', error.message);
    });

    // 打印状态
    setInterval(() => {
      const status = client.getReloadStatus();
      console.log('\n📊 Hot Reload Status:');
      console.log(`  Connection: ${status.connectionStatus}`);
      console.log(`  Reconnects: ${status.reconnectCount}`);
      console.log(`  Function reloads: ${status.functionReloads}`);
      console.log(`  Failed reloads: ${status.failedReloads}`);
      console.log(`  Uptime: ${Math.floor(status.uptime)}s`);
    }, 30000);

    // 演示热重载功能
    setTimeout(() => demonstrateHotReload(client), 10000);

    console.log('\n✅ Server is running!');
    console.log('💡 Modify .js files to trigger hot reload');
    console.log('💡 Use Ctrl+C for graceful shutdown');

  } catch (error) {
    console.error('❌ Failed to start server:', error);
    process.exit(1);
  }
}

// 演示热重载功能
async function demonstrateHotReload(client) {
  console.log('\n🔄 Demonstrating hot reload features...');

  // 1. 函数重载演示
  setTimeout(async () => {
    console.log('\n1. Testing function reload...');

    const newPlayerBanDescriptor = {
      id: 'player.ban',
      version: '1.1.0'
    };

    function handlePlayerBanV2(payload) {
      console.log(`🚫 [V2] Enhanced player ban: ${JSON.stringify(payload)}`);

      return {
        result: 'success',
        message: 'Player banned with enhanced features',
        version: '2.0',
        features: ['account_ban', 'ip_ban', 'device_ban'],
        timestamp: new Date().toISOString()
      };
    }

    try {
      await client.reloadFunction('player.ban', newPlayerBanDescriptor, handlePlayerBanV2);
      console.log('✅ Function reload successful');
    } catch (error) {
      console.error('❌ Function reload failed:', error.message);
    }
  }, 5000);

  // 2. 批量重载演示
  setTimeout(async () => {
    console.log('\n2. Testing batch reload...');

    const functions = {
      'server.status': {
        descriptor: {
          id: 'server.status',
          version: '2.0.0'
        },
        handler: function(payload) {
          return {
            status: 'running',
            version: '2.0',
            uptime: process.uptime(),
            memory: {
              used: Math.round(process.memoryUsage().rss / 1024 / 1024),
              heap: Math.round(process.memoryUsage().heapUsed / 1024 / 1024),
              external: Math.round(process.memoryUsage().external / 1024 / 1024)
            },
            cpu: {
              usage: Math.random() * 100
            },
            connections: {
              active: 42,
              total: 1337
            },
            timestamp: new Date().toISOString()
          };
        }
      }
    };

    try {
      await client.reloadFunctions(functions);
      console.log('✅ Batch reload successful');
    } catch (error) {
      console.error('❌ Batch reload failed:', error.message);
    }
  }, 10000);
}

// 启动应用
if (require.main === module) {
  main().catch(console.error);
}

module.exports = { client, main };