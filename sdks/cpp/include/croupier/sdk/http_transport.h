#pragma once

#include <map>
#include <memory>
#include <string>

namespace croupier::sdk {

// HTTPRequest and HTTPTransport deliberately contain no Provider/Agent
// transport types. They are the narrow boundary used by the public L3
// invoker to call the Server API and make contract tests deterministic.
struct HTTPRequest {
    std::string method;
    std::string url;
    std::map<std::string, std::string> headers;
    std::string body;
    long timeout_ms = 30000;

    bool insecure = false;
    std::string cert_file;
    std::string key_file;
    std::string ca_file;
    std::string server_name;
};

struct HTTPResponse {
    long status_code = 0;
    std::string body;
};

class HTTPTransport {
public:
    virtual ~HTTPTransport() = default;
    virtual HTTPResponse Send(const HTTPRequest& request) = 0;
};

// Creates the libcurl-backed transport used in production. Callers may inject
// a HTTPTransport through InvokerConfig for tests or constrained environments.
std::shared_ptr<HTTPTransport> NewDefaultHTTPTransport();

}  // namespace croupier::sdk
