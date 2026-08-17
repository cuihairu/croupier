#include "croupier/sdk/http_transport.h"

#include <curl/curl.h>

#include <mutex>
#include <stdexcept>
#include <utility>

namespace croupier::sdk {
namespace {

size_t AppendResponse(char* data, size_t size, size_t count, void* target) {
    auto* body = static_cast<std::string*>(target);
    const size_t bytes = size * count;
    body->append(data, bytes);
    return bytes;
}

class CurlHTTPTransport final : public HTTPTransport {
public:
    HTTPResponse Send(const HTTPRequest& request) override {
        EnsureCurlInitialized();
        CURL* handle = curl_easy_init();
        if (handle == nullptr) {
            throw std::runtime_error("initialize Server HTTP transport");
        }

        curl_slist* headers = nullptr;
        try {
            for (const auto& [name, value] : request.headers) {
                headers = curl_slist_append(headers, (name + ": " + value).c_str());
                if (headers == nullptr) {
                    throw std::runtime_error("allocate Server HTTP request headers");
                }
            }

            HTTPResponse response;
            SetOption(handle, CURLOPT_URL, request.url.c_str());
            SetOption(handle, CURLOPT_CUSTOMREQUEST, request.method.c_str());
            SetOption(handle, CURLOPT_HTTPHEADER, headers);
            SetOption(handle, CURLOPT_TIMEOUT_MS, request.timeout_ms);
            SetOption(handle, CURLOPT_CONNECTTIMEOUT_MS, request.timeout_ms);
            SetOption(handle, CURLOPT_WRITEFUNCTION, &AppendResponse);
            SetOption(handle, CURLOPT_WRITEDATA, &response.body);
            SetOption(handle, CURLOPT_NOSIGNAL, 1L);

            if (!request.body.empty()) {
                SetOption(handle, CURLOPT_POSTFIELDS, request.body.c_str());
                SetOption(handle, CURLOPT_POSTFIELDSIZE, static_cast<long>(request.body.size()));
            }
            if (request.insecure) {
                SetOption(handle, CURLOPT_SSL_VERIFYPEER, 0L);
                SetOption(handle, CURLOPT_SSL_VERIFYHOST, 0L);
            }
            if (!request.ca_file.empty()) SetOption(handle, CURLOPT_CAINFO, request.ca_file.c_str());
            if (!request.cert_file.empty()) SetOption(handle, CURLOPT_SSLCERT, request.cert_file.c_str());
            if (!request.key_file.empty()) SetOption(handle, CURLOPT_SSLKEY, request.key_file.c_str());
            const CURLcode code = curl_easy_perform(handle);
            if (code != CURLE_OK) {
                throw std::runtime_error("send Server HTTP request: " + std::string(curl_easy_strerror(code)));
            }
            const CURLcode info_code = curl_easy_getinfo(handle, CURLINFO_RESPONSE_CODE, &response.status_code);
            if (info_code != CURLE_OK) {
                throw std::runtime_error("read Server HTTP response status: " + std::string(curl_easy_strerror(info_code)));
            }
            curl_slist_free_all(headers);
            curl_easy_cleanup(handle);
            return response;
        } catch (...) {
            curl_slist_free_all(headers);
            curl_easy_cleanup(handle);
            throw;
        }
    }

private:
    template <typename T>
    static void SetOption(CURL* handle, CURLoption option, T value) {
        const CURLcode code = curl_easy_setopt(handle, option, value);
        if (code != CURLE_OK) {
            throw std::runtime_error("configure Server HTTP request: " + std::string(curl_easy_strerror(code)));
        }
    }

    static void EnsureCurlInitialized() {
        static std::once_flag initialized;
        static CURLcode result = CURLE_OK;
        std::call_once(initialized, [] { result = curl_global_init(CURL_GLOBAL_DEFAULT); });
        if (result != CURLE_OK) {
            throw std::runtime_error("initialize libcurl: " + std::string(curl_easy_strerror(result)));
        }
    }
};

}  // namespace

std::shared_ptr<HTTPTransport> NewDefaultHTTPTransport() {
    return std::make_shared<CurlHTTPTransport>();
}

}  // namespace croupier::sdk
