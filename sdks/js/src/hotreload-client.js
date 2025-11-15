/**
 * Croupier Node.js SDK with Hot Reload Support
 */

const EventEmitter = require('events');
const fs = require('fs');
const path = require('path');
const chokidar = require('chokidar');

class HotReloadableClient extends EventEmitter {
  constructor(config = {}) {
    super();

    this.config = {
      agentAddr: '127.0.0.1:19090',
      autoReconnect: true,
      reconnectDelay: 5000,
      maxRetryAttempts: 10,
      healthCheckInterval: 30000,
      gracefulShutdownTimeout: 30000,
      fileWatching: {
        enabled: false,
        watchDir: './functions',
        patterns: ['*.js', '*.json', '*.yaml']
      },
      tools: {
        nodemon: true,
        pm2: false,
        moduleReload: true
      },
      ...config
    };

    // 状态管理
    this.isConnected = false;
    this.isReloading = false;
    this.functions = new Map();
    this.reconnectCount = 0;
    this.functionReloads = 0;
    this.configReloads = 0;
    this.failedReloads = 0;
    this.lastReconnectTime = null;

    // 文件监听器
    this.watcher = null;
    this.reconnectTimer = null;
    this.healthCheckTimer = null;

    // 初始化热重载支持
    if (this.config.autoReconnect) {
      this.setupHotReloadSupport();
    }
  }

  /**
   * 注册函数
   */
  registerFunction(descriptor, handler) {
    if (this.isReloading) {
      throw new Error('Cannot register functions during reload operation');
    }

    // 验证函数描述符
    if (!descriptor.id || !descriptor.version) {
      throw new Error('Function descriptor must include id and version');
    }

    // 保存函数定义
    this.functions.set(descriptor.id, {
      descriptor,
      handler,
      registeredAt: new Date()
    });

    console.log(`📝 Registered function: ${descriptor.id} (version: ${descriptor.version})`);
    return this;
  }

  /**
   * 连接到Agent
   */
  async connect() {
    console.log(`🔌 Connecting to Croupier Agent: ${this.config.agentAddr}`);

    try {
      // 这里实现实际的gRPC连接逻辑
      // await this.grpcClient.connect();

      // 注册所有函数
      await this.registerAllFunctions();

      this.isConnected = true;
      console.log('✅ Successfully connected to Agent');

      this.emit('connected');
      return this;
    } catch (error) {
      console.error('❌ Connection failed:', error);
      this.emit('connectionError', error);
      throw error;
    }
  }

  /**
   * 重新加载单个函数
   */
  async reloadFunction(functionId, descriptor, handler) {
    if (this.isReloading) {
      throw new Error('Reload operation already in progress');
    }

    this.isReloading = true;
    console.log(`🔄 Reloading function: ${functionId}`);

    try {
      // 验证函数
      if (descriptor.id !== functionId) {
        throw new Error(`Function ID mismatch: expected ${functionId}, got ${descriptor.id}`);
      }

      // 保存旧函数用于回滚
      const oldFunction = this.functions.get(functionId);

      // 更新函数
      this.functions.set(functionId, {
        descriptor,
        handler,
        reloadedAt: new Date()
      });

      // 清除模块缓存（如果启用）
      if (this.config.tools.moduleReload) {
        this.clearModuleCache(functionId);
      }

      // 重新注册到Agent
      // await this.grpcClient.registerFunction(descriptor, handler);

      this.functionReloads++;
      console.log(`✅ Function ${functionId} reloaded successfully`);

      this.emit('functionReloaded', functionId, descriptor);
      return this;
    } catch (error) {
      this.failedReloads++;
      console.error(`❌ Failed to reload function ${functionId}:`, error);
      this.emit('reloadError', functionId, error);
      throw error;
    } finally {
      this.isReloading = false;
    }
  }

  /**
   * 批量重载函数
   */
  async reloadFunctions(functions) {
    if (this.isReloading) {
      throw new Error('Reload operation already in progress');
    }

    this.isReloading = true;
    console.log(`🔄 Batch reloading ${Object.keys(functions).length} functions`);

    const results = [];
    const errors = [];

    try {
      for (const [functionId, { descriptor, handler }] of Object.entries(functions)) {
        try {
          await this.reloadFunction(functionId, descriptor, handler);
          results.push(functionId);
        } catch (error) {
          errors.push({ functionId, error });
        }
      }

      if (errors.length > 0) {
        const errorMsg = `Failed to reload ${errors.length} out of ${Object.keys(functions).length} functions`;
        console.error(errorMsg);
        throw new Error(errorMsg);
      }

      console.log(`✅ Successfully reloaded all ${results.length} functions`);
      return this;
    } finally {
      this.isReloading = false;
    }
  }

  /**
   * 重载配置
   */
  async reloadConfig(newConfig) {
    console.log('🔄 Reloading client configuration');

    // 合并配置
    this.config = { ...this.config, ...newConfig };

    this.configReloads++;
    console.log('✅ Configuration reloaded successfully');

    this.emit('configReloaded', this.config);
    return this;
  }

  /**
   * 获取重载状态
   */
  getReloadStatus() {
    return {
      reconnectCount: this.reconnectCount,
      lastReconnectTime: this.lastReconnectTime,
      functionReloads: this.functionReloads,
      configReloads: this.configReloads,
      failedReloads: this.failedReloads,
      connectionStatus: this.isConnected ? (this.isReloading ? 'reloading' : 'connected') : 'disconnected',
      functionsCount: this.functions.size,
      uptime: process.uptime()
    };
  }

  /**
   * 重新连接
   */
  async reconnect() {
    console.log('🔄 Attempting to reconnect...');

    try {
      // 断开当前连接
      await this.disconnect();

      // 重新连接
      await this.connect();

      this.reconnectCount++;
      this.lastReconnectTime = new Date();

      console.log('✅ Reconnection successful');
      this.emit('reconnected');
      return this;
    } catch (error) {
      this.failedReloads++;
      console.error('❌ Reconnection failed:', error);
      this.emit('reconnectError', error);
      throw error;
    }
  }

  /**
   * 优雅关闭
   */
  async gracefulShutdown(timeout = this.config.gracefulShutdownTimeout) {
    console.log(`🛑 Starting graceful shutdown (timeout: ${timeout}ms)`);

    const shutdownPromise = new Promise(async (resolve) => {
      // 停止文件监听
      this.stopFileWatching();

      // 停止定时器
      if (this.healthCheckTimer) {
        clearInterval(this.healthCheckTimer);
      }
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
      }

      // 断开连接
      await this.disconnect();

      console.log('✅ Graceful shutdown completed');
      resolve();
    });

    // 设置超时
    return Promise.race([
      shutdownPromise,
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Shutdown timeout')), timeout)
      )
    ]);
  }

  /**
   * 设置热重载支持
   */
  setupHotReloadSupport() {
    // 设置Nodemon支持
    if (this.config.tools.nodemon) {
      this.setupNodemonSupport();
    }

    // 设置PM2支持
    if (this.config.tools.pm2) {
      this.setupPM2Support();
    }

    // 启动自动重连
    this.startAutoReconnect();

    // 启动文件监听
    if (this.config.fileWatching.enabled) {
      this.startFileWatching();
    }

    console.log('🔥 Hot reload support enabled');
  }

  /**
   * 设置Nodemon支持
   */
  setupNodemonSupport() {
    // 监听SIGUSR2信号（Nodemon重启信号）
    process.once('SIGUSR2', () => {
      console.log('📡 Received Nodemon restart signal');
      this.gracefulShutdown().then(() => {
        process.kill(process.pid, 'SIGUSR2');
      });
    });

    // 监听退出信号
    process.on('SIGINT', () => this.gracefulShutdown());
    process.on('SIGTERM', () => this.gracefulShutdown());
  }

  /**
   * 设置PM2支持
   */
  setupPM2Support() {
    process.on('message', (msg) => {
      if (msg === 'shutdown') {
        console.log('📡 Received PM2 shutdown signal');
        this.gracefulShutdown().then(() => {
          process.exit(0);
        });
      }
    });
  }

  /**
   * 启动自动重连
   */
  startAutoReconnect() {
    this.healthCheckTimer = setInterval(async () => {
      if (!this.isConnected && !this.isReloading) {
        await this.attemptReconnect();
      }
    }, this.config.healthCheckInterval);
  }

  /**
   * 尝试重连
   */
  async attemptReconnect() {
    let delay = this.config.reconnectDelay;

    for (let attempt = 1; attempt <= this.config.maxRetryAttempts; attempt++) {
      console.log(`🔄 Reconnection attempt ${attempt}/${this.config.maxRetryAttempts}`);

      try {
        await this.reconnect();
        return; // 成功重连，退出循环
      } catch (error) {
        console.error(`❌ Reconnection attempt ${attempt} failed:`, error.message);

        if (attempt < this.config.maxRetryAttempts) {
          await new Promise(resolve => setTimeout(resolve, delay));
          // 指数退避
          delay = Math.min(delay * 1.5, 60000);
        }
      }
    }

    console.error('❌ All reconnection attempts failed');
    this.emit('reconnectFailed');
  }

  /**
   * 启动文件监听
   */
  startFileWatching() {
    if (!this.config.fileWatching.watchDir) {
      return;
    }

    console.log(`👀 Watching directory: ${this.config.fileWatching.watchDir}`);

    this.watcher = chokidar.watch(this.config.fileWatching.watchDir, {
      ignored: /(^|[\/\\])\../, // 忽略点文件
      persistent: true
    });

    this.watcher.on('change', (path) => {
      console.log(`📁 File changed: ${path}`);

      // 根据文件类型触发不同的重载行为
      if (path.endsWith('.js')) {
        this.handleJSFileChange(path);
      } else if (path.endsWith('.json') || path.endsWith('.yaml')) {
        this.handleConfigFileChange(path);
      }
    });

    this.watcher.on('error', (error) => {
      console.error('❌ File watcher error:', error);
    });
  }

  /**
   * 处理JS文件变更
   */
  handleJSFileChange(filePath) {
    if (this.config.tools.moduleReload) {
      // 清除模块缓存
      delete require.cache[require.resolve(path.resolve(filePath))];
      console.log(`🗑️ Cleared module cache for: ${filePath}`);
    }
  }

  /**
   * 处理配置文件变更
   */
  handleConfigFileChange(filePath) {
    try {
      console.log(`🔄 Reloading configuration from: ${filePath}`);
      // 这里可以实现配置文件重载逻辑
    } catch (error) {
      console.error('❌ Failed to reload configuration:', error);
    }
  }

  /**
   * 停止文件监听
   */
  stopFileWatching() {
    if (this.watcher) {
      this.watcher.close();
      this.watcher = null;
      console.log('👀 File watching stopped');
    }
  }

  /**
   * 清除模块缓存
   */
  clearModuleCache(functionId) {
    // 简单的缓存清除策略
    const cacheKeys = Object.keys(require.cache);
    const functionModules = cacheKeys.filter(key =>
      key.includes(functionId) || key.includes('function')
    );

    functionModules.forEach(key => {
      delete require.cache[key];
    });

    if (functionModules.length > 0) {
      console.log(`🗑️ Cleared ${functionModules.length} module cache entries for ${functionId}`);
    }
  }

  /**
   * 注册所有函数到Agent
   */
  async registerAllFunctions() {
    console.log(`📋 Registering ${this.functions.size} functions with Agent`);

    for (const [functionId, { descriptor, handler }] of this.functions) {
      // 这里实现实际的gRPC注册逻辑
      console.log(`  - ${functionId} (${descriptor.version})`);
    }
  }

  /**
   * 断开连接
   */
  async disconnect() {
    if (this.isConnected) {
      // 实现实际的断开连接逻辑
      this.isConnected = false;
      console.log('🔌 Disconnected from Agent');
    }
  }
}

module.exports = HotReloadableClient;