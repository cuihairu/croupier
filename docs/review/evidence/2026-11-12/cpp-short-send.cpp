// Inject a legal short send into the actual C++ SDK response writer.
#include <bits/stdc++.h>
#include <sys/socket.h>
#define private public
#include "croupier/sdk/tcp_transport.h"
#undef private
static size_t requested = 0;
static size_t accepted = 0;
extern "C" ssize_t __wrap_send(int, const void*, size_t length, int) {
    requested = length;
    accepted = 5;
    return static_cast<ssize_t>(accepted);
}
int main() {
    croupier::sdk::TCPTransport::WriteResponseOnSocket(
        99, croupier::sdk::protocol::MSG_INVOKE_RESPONSE, 7, std::vector<uint8_t>(128, 42));
    if (requested != 140 || accepted != 5) return 1;
    std::cout << "REPRODUCED: response writer requested " << requested
              << " bytes, send accepted " << accepted
              << "; writer returned without retry/error/connection invalidation\n";
}
