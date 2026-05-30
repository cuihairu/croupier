#pragma once

#include <chrono>
#include <ctime>
#include <iomanip>
#include <iostream>
#include <mutex>
#include <regex>
#include <sstream>
#include <string>

// Optional: Use spdlog if available
// Uncomment to use spdlog instead of simple logger
// #define CROUPIER_USE_SPDLOG

#ifdef CROUPIER_USE_SPDLOG
#include <spdlog/sinks/stdout_color_sinks.h>
#include <spdlog/spdlog.h>
#endif

namespace croupier {
namespace sdk {

// ========== Sensitive Data Masking Utilities ==========
// These utilities help comply with SDK specification requirements for log data masking

// Mask a sensitive value (token, password, key) for logging
// Format: abc...xyz (shows first 3 and last 3 characters)
inline std::string MaskSensitive(const std::string& value) {
    if (value.empty())
        return "";
    const size_t min_visible = 3;
    if (value.length() <= min_visible * 2) {
        return std::string(value.length(), '*');
    }
    return value.substr(0, min_visible) + "..." + value.substr(value.length() - min_visible);
}

// Mask a value but preserve its length (all asterisks)
inline std::string MaskFully(const std::string& value) {
    if (value.empty())
        return "";
    return std::string(value.length(), '*');
}

// Mask JSON value by key (for logging JSON payloads)
inline std::string MaskJsonSensitive(const std::string& json, const std::vector<std::string>& keys_to_mask) {
    std::string result = json;
    for (const auto& key : keys_to_mask) {
        std::regex pattern("\"" + key + "\"\\s*:\\s*\"([^\"]+)\"");
        std::string replacement = "\"" + key + "\":\"" + MaskSensitive("$1") + "\"";
        // Simple replacement - in production, use a proper JSON parser
        result = std::regex_replace(result, pattern, "\"" + key + "\":\"***masked***\"");
    }
    return result;
}

// Simple logger for Croupier SDK
// Can be configured to use spdlog if CROUPIER_USE_SPDLOG is defined
class Logger {
public:
    enum class Level { DEBUG = 0, INFO = 1, WARN = 2, ERR = 3, OFF = 4 };

    static Logger& GetInstance() {
        static Logger instance;
        return instance;
    }

    void SetLevel(Level level) { level_ = level; }

    void SetLevelFromString(const std::string& level) {
        if (level == "DEBUG" || level == "debug") {
            SetLevel(Level::DEBUG);
        } else if (level == "INFO" || level == "info") {
            SetLevel(Level::INFO);
        } else if (level == "WARN" || level == "warn") {
            SetLevel(Level::WARN);
        } else if (level == "ERROR" || level == "error") {
            SetLevel(Level::ERR);
        } else if (level == "OFF" || level == "off") {
            SetLevel(Level::OFF);
        }
    }

    void Disable(bool disable) {
        if (disable) {
            SetLevel(Level::OFF);
        } else {
            SetLevel(Level::INFO);
        }
    }

    bool IsEnabled(Level level) const { return level >= level_; }

    void Log(Level level, const std::string& component, const std::string& message) {
        if (!IsEnabled(level)) {
            return;
        }

        std::lock_guard<std::mutex> lock(mutex_);

#ifdef CROUPIER_USE_SPDLOG
        if (spdlog_logger_) {
            switch (level) {
            case Level::DEBUG:
                spdlog_logger_->debug("[{}] {}", component, message);
                break;
            case Level::INFO:
                spdlog_logger_->info("[{}] {}", component, message);
                break;
            case Level::WARN:
                spdlog_logger_->warn("[{}] {}", component, message);
                break;
            case Level::ERR:
                spdlog_logger_->error("[{}] {}", component, message);
                break;
            default:
                break;
            }
            return;
        }
#endif

        // Simple console logging
        std::string level_str = LevelToString(level);
        std::string timestamp = GetTimestamp();

        std::cout << timestamp << " [" << level_str << "] [" << component << "] " << message << std::endl;
    }

    // Log with sensitive data masking (for tokens, passwords, etc.)
    void LogMasked(Level level, const std::string& component, const std::string& message,
                   const std::string& sensitive_value) {
        if (!IsEnabled(level)) {
            return;
        }
        Log(level, component, message + " (masked: " + MaskSensitive(sensitive_value) + ")");
    }

    // Convenience methods
    void Debug(const std::string& component, const std::string& message) { Log(Level::DEBUG, component, message); }

    void Info(const std::string& component, const std::string& message) { Log(Level::INFO, component, message); }

    void Warn(const std::string& component, const std::string& message) { Log(Level::WARN, component, message); }

    void Error(const std::string& component, const std::string& message) { Log(Level::ERR, component, message); }

#ifdef CROUPIER_USE_SPDLOG
    void SetSpdlogLogger(std::shared_ptr<spdlog::logger> logger) { spdlog_logger_ = logger; }
#endif

private:
    Logger() : level_(Level::INFO) {
#ifdef CROUPIER_USE_SPDLOG
        try {
            spdlog_logger_ = spdlog::stdout_color_mt("croupier");
            spdlog_logger_->set_level(spdlog::level::info);
            spdlog_logger_->set_pattern("[%Y-%m-%d %H:%M:%S.%e] [%^%l%$] [%n] %v");
        } catch (const spdlog::spdlog_ex& ex) {
            std::cerr << "Failed to initialize spdlog: " << ex.what() << std::endl;
        }
#endif
    }

    std::string LevelToString(Level level) const {
        switch (level) {
        case Level::DEBUG:
            return "DEBUG";
        case Level::INFO:
            return "INFO";
        case Level::WARN:
            return "WARN";
        case Level::ERR:
            return "ERROR";
        default:
            return "UNKNOWN";
        }
    }

    std::string GetTimestamp() const {
        auto now = std::chrono::system_clock::now();
        auto time_t = std::chrono::system_clock::to_time_t(now);
        auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(now.time_since_epoch()) % 1000;

        std::stringstream ss;
        ss << std::put_time(std::gmtime(&time_t), "%Y-%m-%dT%H:%M:%S");
        ss << '.' << std::setfill('0') << std::setw(3) << ms.count() << 'Z';
        return ss.str();
    }

    Level level_;
    std::mutex mutex_;
#ifdef CROUPIER_USE_SPDLOG
    std::shared_ptr<spdlog::logger> spdlog_logger_;
#endif
};

// Convenience macros for logging
#define CROUPIER_LOG_DEBUG(component, message) croupier::sdk::Logger::GetInstance().Debug(component, message)

#define CROUPIER_LOG_INFO(component, message) croupier::sdk::Logger::GetInstance().Info(component, message)

#define CROUPIER_LOG_WARN(component, message) croupier::sdk::Logger::GetInstance().Warn(component, message)

#define CROUPIER_LOG_ERROR(component, message) croupier::sdk::Logger::GetInstance().Error(component, message)

// Convenience macro for logging with masked sensitive value
#define CROUPIER_LOG_INFO_MASKED(component, message, sensitive_value)                                      \
    croupier::sdk::Logger::GetInstance().LogMasked(croupier::sdk::Logger::Level::INFO, component, message, \
                                                   sensitive_value)

// Scoped logger for specific component
class ComponentLogger {
public:
    explicit ComponentLogger(const std::string& component) : component_(component) {}

    void Debug(const std::string& message) const { CROUPIER_LOG_DEBUG(component_, message); }

    void Info(const std::string& message) const { CROUPIER_LOG_INFO(component_, message); }

    void Warn(const std::string& message) const { CROUPIER_LOG_WARN(component_, message); }

    void Error(const std::string& message) const { CROUPIER_LOG_ERROR(component_, message); }

private:
    std::string component_;
};

}  // namespace sdk
}  // namespace croupier
