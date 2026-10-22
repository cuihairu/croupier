# Quick Start

```cpp
#include "croupier/sdk/croupier_client.h"

int main() {
    croupier::sdk::ClientConfig config;
    config.game_id = "my-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19091";

    croupier::sdk::CroupierClient client(config);
    client.Connect();
    client.Serve();
}
```

## Next Steps
