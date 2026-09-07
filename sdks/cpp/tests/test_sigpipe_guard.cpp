// SIGPIPE guard for the whole test binary.
//
// The SDK's TCP transport intentionally writes to sockets that the peer may
// have already reset (heartbeat probing, drain, failover). On Linux such a
// write raises SIGPIPE and kills the process before the transport's error
// handling can run. Product binaries are expected to decide their own SIGPIPE
// policy; the test binary ignores the signal so the EPIPE paths are exercised
// deterministically instead of crashing the suite.
#include <csignal>

namespace croupier_tests {

namespace {
struct SigpipeGuard {
    SigpipeGuard() { std::signal(SIGPIPE, SIG_IGN); }
};
const SigpipeGuard g_sigpipe_guard;
}  // namespace

}  // namespace croupier_tests
