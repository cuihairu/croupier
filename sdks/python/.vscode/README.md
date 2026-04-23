# Croupier Python SDK - 开发环境设置

## 首次设置（必需）

在首次调试或运行示例前，需要在开发模式下安装 SDK：

```bash
# 进入项目目录
cd croupier-sdk-python

# 以可编辑模式安装（开发时修改代码立即生效）
pip install -e .

# 或同时安装开发依赖
pip install -e ".[dev]"
```

## 验证安装

```bash
# 验证可以导入 croupier 模块
python -c "from croupier import CroupierClient; print('安装成功！')"

# 运行测试
pytest tests -v
```

## VS Code 调试配置

| 配置名称 | 用途 |
|---------|------|
| `Python: 当前文件` | 调试当前打开的 Python 文件 |
| `Python: 示例 - main.py` | 调试同步客户端示例 |
| `Python: 示例 - invoker (async)` | 调试异步调用者示例 |
| `Python: 示例 - invoker (sync)` | 调试同步调用者示例 |
| `Python: pytest - 当前文件` | 运行当前测试文件 |
| `Python: pytest - 所有测试` | 运行所有测试 |
| `Python: pytest - 特定测试` | 运行匹配名称的测试 |
| `Python: pytest - 覆盖率` | 运行测试并显示覆盖率 |

## 常见问题

### ImportError: No module named 'croupier'

执行首次设置中的 `pip install -e .` 命令。

### 示例运行需要服务端

示例程序需要连接到 Croupier Agent 和 Control 服务：

- `agent_addr`: 默认 `127.0.0.1:19090`
- `control_addr`: 默认 `127.0.0.1:18080`

请确保服务端已启动，或修改示例中的连接地址。
