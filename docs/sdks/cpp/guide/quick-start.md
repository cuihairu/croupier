# 快速开始

```cpp
#include "croupier/sdk/croupier_client.h"

int main() {
    croupier::sdk::ClientConfig config;
    config.game_id = "my-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    croupier::sdk::CroupierClient client(config);
    client.Connect();
    client.Serve();
}
```

## 下一步

- [函数注册](./functions)
- [Client Config](/sdks/cpp/configuration/client-config)
