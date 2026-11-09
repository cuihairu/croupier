// Legacy Agent TCP invoker, retained verbatim as an implementation
// reference (previously an #if 0 block inside croupier_client.cpp).
// Not compiled: this file is not listed in CMake SDK_SOURCES.

// Invoker Implementation
#if 0  // Legacy Agent TCP invoker retained only as an implementation reference.
class CroupierInvoker::Impl {
public:
    struct LocalJobState {
        std::string task_id;
        std::string function_id;
        std::string payload;
        std::vector<TaskEvent> events;
        std::atomic<bool> done{false};
        std::atomic<bool> cancelled{false};
        std::thread worker;
    };

    InvokerConfig config_;
    ReconnectConfig reconnect_config_;
    RetryConfig retry_config_;
    std::map<std::string, std::map<std::string, std::string>> schemas_;
    std::unique_ptr<TCPTransport> transport_;
    std::atomic<bool> connected_{false};
    std::atomic<uint64_t> next_task_id_{1};
    std::mutex transport_mutex_;
    std::mutex jobs_mutex_;
    std::unordered_map<std::string, std::shared_ptr<LocalJobState>> jobs_;

    // Reconnection state
    std::atomic<bool> is_reconnecting_{false};
    std::atomic<int> reconnect_attempts_{0};
    std::atomic<bool> should_stop_reconnecting_{false};
    std::thread reconnect_thread_;
    std::mutex reconnect_mutex_;  // Protects reconnect_thread_ access
    std::string last_error_;

    // Close state
    std::atomic<bool> closed_{false};

    explicit Impl(const InvokerConfig& config) : config_(config) {
        // ========== Initialize Logger Configuration ==========
        auto& logger = Logger::GetInstance();

        if (config_.disable_logging) {
            logger.SetLevel(Logger::Level::OFF);
        } else if (config_.debug_logging) {
            logger.SetLevel(Logger::Level::DEBUG);
        } else {
            logger.SetLevelFromString(config_.log_level);
        }

        // Set default reconnect config
        reconnect_config_.enabled = true;
        reconnect_config_.max_attempts = 0;  // Infinite
        reconnect_config_.initial_delay_ms = 1000;
        reconnect_config_.max_delay_ms = 30000;
        reconnect_config_.backoff_multiplier = 2.0;
        reconnect_config_.jitter_factor = 0.2;

        // Initialize retry config from config or use defaults
        retry_config_ = config_.retry;
        if (!retry_config_.enabled) {
            // Use default retry config if not set
            retry_config_.enabled = true;
            retry_config_.max_attempts = 3;
            retry_config_.initial_delay_ms = 100;
            retry_config_.max_delay_ms = 5000;
            retry_config_.backoff_multiplier = 2.0;
            retry_config_.jitter_factor = 0.1;
            if (retry_config_.retryable_status_codes.empty()) {
                retry_config_.retryable_status_codes = {14, 13, 2, 10, 4};  // Default codes
            }
        }
    }

    ~Impl() { Close(); }

    bool Connect() {
        bool result = connectInternal();
        if (!result && IsConnectionError()) {
            ScheduleReconnectIfNeeded();
        }
        return result;
    }

    // Internal connect method that doesn't trigger reconnection
    bool connectInternal() {
        if (connected_)
            return true;

        SDK_LOG_INFO("Connecting to server/agent at: " + config_.address);
        if (config_.address.empty()) {
            last_error_ = "connection address is empty";
            connected_ = false;
            return false;
        }

        if (!IsTCPAddress(config_.address)) {
            last_error_.clear();
            connected_ = true;
            SDK_LOG_INFO("Using local fallback mode for non-TCP address: " + config_.address);
            return true;
        }

        try {
            const auto server_address = ParseTCPAddress(config_.address);
            auto transport =
                std::make_unique<TCPTransport>(server_address.host, server_address.port, config_.timeout_seconds * 1000);
            transport->SetConnectTimeout(config_.connect_timeout_seconds * 1000);
            transport->Connect();
            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                transport_ = std::move(transport);
            }
            last_error_.clear();
            connected_ = true;
            SDK_LOG_INFO("Connected to: " + server_address.display);
            return true;
        } catch (const std::exception& e) {
            last_error_ = e.what();
            connected_ = false;
            SDK_LOG_ERROR("Failed to connect: " + last_error_);
            return false;
        }
    }

    std::string Invoke(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        if (!connected_ && !connectInternal()) {
            if (IsConnectionError()) {
                ScheduleReconnectIfNeeded();
            }
            throw std::runtime_error("Not connected to server");
        }

        // Client-side validation
        auto it = schemas_.find(function_id);
        if (it != schemas_.end()) {
            if (!utils::ValidateJSON(payload, it->second)) {
                throw std::runtime_error("Payload validation failed for function: " + function_id);
            }
        }

        // Get retry config (use options retry if provided, otherwise use config retry)
        const RetryConfig& retry_config = options.retry.has_value() ? *options.retry : retry_config_;

        // If retry is disabled, execute directly
        if (!retry_config.enabled) {
            return invokeInternal(function_id, payload, options);
        }

        // Execute with retry
        int max_attempts = retry_config.max_attempts;
        std::string last_error;

        for (int attempt = 0; attempt < max_attempts; ++attempt) {
            try {
                return invokeInternal(function_id, payload, options);
            } catch (const std::exception& e) {
                last_error = e.what();

                // Check if this error is retryable and not the last attempt
                if (attempt >= max_attempts - 1) {
                    throw std::runtime_error("Invoke failed after " + std::to_string(max_attempts) +
                                             " attempts: " + last_error);
                }

                // Check if error is retryable (simplified check)
                bool is_retryable = last_error.find("UNAVAILABLE") != std::string::npos ||
                                    last_error.find("INTERNAL") != std::string::npos ||
                                    last_error.find("DEADLINE") != std::string::npos ||
                                    last_error.find("connection") != std::string::npos ||
                                    last_error.find("timeout") != std::string::npos;

                if (!is_retryable) {
                    throw std::runtime_error("Invoke failed with non-retryable error: " + last_error);
                }

                // Connection errors should trigger reconnection
                if (IsConnectionError() && reconnect_config_.enabled) {
                    connected_ = false;
                    ScheduleReconnectIfNeeded();
                }

                // Calculate delay and wait
                int delay = CalculateRetryDelay(attempt);
                std::cout << "Invocation attempt " << (attempt + 1) << " failed, retrying in " << delay
                          << " ms: " << last_error << '\n';
                std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            }
        }

        throw std::runtime_error("Invoke failed after " + std::to_string(max_attempts) + " attempts: " + last_error);
    }

    std::string invokeInternal(const std::string& function_id, const std::string& payload,
                               const InvokeOptions& options) {
        if (!IsTCPAddress(config_.address)) {
            (void)options;
            std::stringstream response;
            response << "{\"status\":\"success\",\"function_id\":\"" << EscapeJsonString(function_id)
                     << "\",\"payload\":" << (payload.empty() ? "null" : payload) << "}";
            return response.str();
        }

        ::croupier::sdk::v1::InvokeRequest req;
        req.set_function_id(function_id);
        req.set_idempotency_key(options.idempotency_key.empty() ? utils::NewIdempotencyKey() : options.idempotency_key);
        req.set_payload(payload);

        for (const auto& [key, value] : config_.headers) {
            (*req.mutable_metadata())[key] = value;
        }
        for (const auto& [key, value] : options.metadata) {
            (*req.mutable_metadata())[key] = value;
        }
        if (!config_.auth_token.empty() && req.metadata().find("Authorization") == req.metadata().end()) {
            (*req.mutable_metadata())["Authorization"] = "Bearer " + config_.auth_token;
        }
        if (!config_.game_id.empty()) {
            (*req.mutable_metadata())["X-Game-ID"] = config_.game_id;
        }
        if (!config_.env.empty()) {
            (*req.mutable_metadata())["X-Env"] = config_.env;
        }
        if (!options.route.empty()) {
            (*req.mutable_metadata())["route"] = options.route;
        }
        if (!options.target_service_id.empty()) {
            (*req.mutable_metadata())["target_service_id"] = options.target_service_id;
        }
        if (!options.hash_key.empty()) {
            (*req.mutable_metadata())["hash_key"] = options.hash_key;
        }
        if (!options.trace_id.empty()) {
            (*req.mutable_metadata())["trace_id"] = options.trace_id;
        }

        std::lock_guard<std::mutex> lock(transport_mutex_);
        if (!transport_ || !transport_->IsConnected()) {
            throw std::runtime_error("Not connected to server");
        }

        auto [_, response_body] = transport_->Call(protocol::MSG_INVOKE_REQUEST, SerializeMessage(req));
        auto response = ParseMessage<::croupier::sdk::v1::InvokeResponse>(response_body, "InvokeResponse");
        return response.payload();
    }

    std::string StartTask(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        if (!connected_ && !connectInternal()) {
            if (IsConnectionError()) {
                ScheduleReconnectIfNeeded();
            }
            throw std::runtime_error("Not connected to server");
        }

        // Client-side validation
        auto it = schemas_.find(function_id);
        if (it != schemas_.end()) {
            if (!utils::ValidateJSON(payload, it->second)) {
                throw std::runtime_error("Payload validation failed for function: " + function_id);
            }
        }

        // Get retry config (use options retry if provided, otherwise use config retry)
        const RetryConfig& retry_config = options.retry.has_value() ? *options.retry : retry_config_;

        // If retry is disabled, execute directly
        if (!retry_config.enabled) {
            return startJobInternal(function_id, payload, options);
        }

        // Execute with retry
        int max_attempts = retry_config.max_attempts;
        std::string last_error;

        for (int attempt = 0; attempt < max_attempts; ++attempt) {
            try {
                return startJobInternal(function_id, payload, options);
            } catch (const std::exception& e) {
                last_error = e.what();

                // Check if this error is retryable and not the last attempt
                if (attempt >= max_attempts - 1) {
                    throw std::runtime_error("StartTask failed after " + std::to_string(max_attempts) +
                                             " attempts: " + last_error);
                }

                // Check if error is retryable (simplified check)
                bool is_retryable = last_error.find("UNAVAILABLE") != std::string::npos ||
                                    last_error.find("INTERNAL") != std::string::npos ||
                                    last_error.find("DEADLINE") != std::string::npos ||
                                    last_error.find("connection") != std::string::npos ||
                                    last_error.find("timeout") != std::string::npos;

                if (!is_retryable) {
                    throw std::runtime_error("StartTask failed with non-retryable error: " + last_error);
                }

                // Connection errors should trigger reconnection
                if (IsConnectionError() && reconnect_config_.enabled) {
                    connected_ = false;
                    ScheduleReconnectIfNeeded();
                }

                // Calculate delay and wait
                int delay = CalculateRetryDelay(attempt);
                std::cout << "StartTask attempt " << (attempt + 1) << " failed, retrying in " << delay
                          << " ms: " << last_error << '\n';
                std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            }
        }

        throw std::runtime_error("StartTask failed after " + std::to_string(max_attempts) + " attempts: " + last_error);
    }

    std::string startJobInternal(const std::string& function_id, const std::string& payload,
                                 const InvokeOptions& options) {
        if (IsTCPAddress(config_.address)) {
            ::croupier::sdk::v1::InvokeRequest req;
            req.set_function_id(function_id);
            req.set_idempotency_key(options.idempotency_key.empty() ? utils::NewIdempotencyKey() : options.idempotency_key);
            req.set_payload(payload);

            for (const auto& [key, value] : config_.headers) {
                (*req.mutable_metadata())[key] = value;
            }
            for (const auto& [key, value] : options.metadata) {
                (*req.mutable_metadata())[key] = value;
            }
            if (!config_.auth_token.empty() && req.metadata().find("Authorization") == req.metadata().end()) {
                (*req.mutable_metadata())["Authorization"] = "Bearer " + config_.auth_token;
            }
            if (!config_.game_id.empty()) {
                (*req.mutable_metadata())["X-Game-ID"] = config_.game_id;
            }
            if (!config_.env.empty()) {
                (*req.mutable_metadata())["X-Env"] = config_.env;
            }
            if (!options.route.empty()) {
                (*req.mutable_metadata())["route"] = options.route;
            }
            if (!options.target_service_id.empty()) {
                (*req.mutable_metadata())["target_service_id"] = options.target_service_id;
            }
            if (!options.hash_key.empty()) {
                (*req.mutable_metadata())["hash_key"] = options.hash_key;
            }
            if (!options.trace_id.empty()) {
                (*req.mutable_metadata())["trace_id"] = options.trace_id;
            }

            std::vector<uint8_t> response_body;
            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                if (!transport_ || !transport_->IsConnected()) {
                    throw std::runtime_error("Not connected to server");
                }
                auto response = transport_->Call(protocol::MSG_START_TASK_REQUEST, SerializeMessage(req));
                response_body = std::move(response.second);
            }

            auto response = ParseMessage<::croupier::sdk::v1::StartTaskResponse>(response_body, "StartTaskResponse");
            if (response.task_id().empty()) {
                throw std::runtime_error("StartTask response did not include job ID");
            }

            auto state = std::make_shared<LocalJobState>();
            state->task_id = response.task_id();
            state->function_id = function_id;
            state->payload = payload;

            TaskEvent started_event;
            started_event.event_type = "started";
            started_event.task_id = response.task_id();
            started_event.message = "Job started";
            started_event.progress = 0;
            started_event.done = false;
            state->events.push_back(started_event);

            {
                std::lock_guard<std::mutex> lock(jobs_mutex_);
                jobs_[state->task_id] = state;
            }

            return response.task_id();
        }

        std::cout << "Starting job for function: " << function_id << '\n';
        std::string task_id = "job-" + std::to_string(next_task_id_.fetch_add(1));
        auto job = std::make_shared<LocalJobState>();
        job->task_id = task_id;
        job->function_id = function_id;
        job->payload = payload;

        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            jobs_[task_id] = job;
        }

        job->worker = std::thread([this, job, options]() {
            if (job->cancelled) {
                return;
            }

            TaskEvent started;
            started.event_type = "started";
            started.task_id = job->task_id;
            started.payload = "{\"status\":\"started\"}";
            appendTaskEvent(job, started);

            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            if (job->cancelled) {
                return;
            }

            TaskEvent progress;
            progress.event_type = "progress";
            progress.task_id = job->task_id;
            progress.progress = 50;
            progress.payload = "{\"progress\":50}";
            appendTaskEvent(job, progress);

            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            if (job->cancelled) {
                return;
            }

            try {
                const std::string result = invokeInternal(job->function_id, job->payload, options);
                if (job->cancelled) {
                    return;
                }

                TaskEvent completed;
                completed.event_type = "completed";
                completed.task_id = job->task_id;
                completed.payload = result;
                completed.progress = 100;
                completed.done = true;
                appendTaskEvent(job, completed);
            } catch (const std::exception& e) {
                TaskEvent error;
                error.event_type = "failed";
                error.task_id = job->task_id;
                error.error = e.what();
                error.done = true;
                appendTaskEvent(job, error);
            }
        });

        std::cout << "Job started: " << task_id << '\n';
        return task_id;
    }

    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id) {
        return std::async(std::launch::async, [this, task_id]() {
            if (!connected_ && !connectInternal()) {
                if (IsConnectionError()) {
                    ScheduleReconnectIfNeeded();
                }
                TaskEvent error_event;
                error_event.event_type = "error";
                error_event.task_id = task_id;
                error_event.error = "Not connected to server";
                error_event.message = error_event.error;
                error_event.done = true;
                return std::vector<TaskEvent>{error_event};
            }

            if (IsTCPAddress(config_.address)) {
                std::vector<TaskEvent> events;
                {
                    std::lock_guard<std::mutex> lock(jobs_mutex_);
                    auto it = jobs_.find(task_id);
                    if (it != jobs_.end()) {
                        events.insert(events.end(), it->second->events.begin(), it->second->events.end());
                        if (!events.empty() && IsTerminalTaskEvent(events.back())) {
                            jobs_.erase(it);
                            return events;
                        }
                    }
                }

                for (int attempt = 0; attempt < 120; ++attempt) {
                    ::croupier::sdk::v1::TaskStreamRequest req;
                    req.set_task_id(task_id);

                    std::vector<uint8_t> response_body;
                    {
                        std::lock_guard<std::mutex> lock(transport_mutex_);
                        if (!transport_ || !transport_->IsConnected()) {
                            TaskEvent error_event;
                            error_event.task_id = task_id;
                            error_event.error = "Connection lost while streaming job";
                            error_event.done = true;
                            events.push_back(error_event);
                            return events;
                        }
                        auto response = transport_->Call(protocol::MSG_STREAM_TASK_REQUEST, SerializeMessage(req));
                        response_body = std::move(response.second);
                    }

                    auto proto_event = ParseMessage<::croupier::sdk::v1::TaskEvent>(response_body, "TaskEvent");
                    TaskEvent event = ToTaskEvent(task_id, proto_event);
                    if (events.empty() || !SameTaskEvent(events.back(), event)) {
                        events.push_back(event);
                    }

                    {
                        std::lock_guard<std::mutex> lock(jobs_mutex_);
                        auto& state = jobs_[task_id];
                        if (!state) {
                            state = std::make_shared<LocalJobState>();
                            state->task_id = task_id;
                        }
                        state->events = events;
                        if (IsTerminalTaskEvent(event)) {
                            state->done = true;
                        }
                    }

                    if (IsTerminalTaskEvent(event)) {
                        std::lock_guard<std::mutex> lock(jobs_mutex_);
                        jobs_.erase(task_id);
                        return events;
                    }

                    std::this_thread::sleep_for(std::chrono::milliseconds(500));
                }

                TaskEvent timeout_event;
                timeout_event.event_type = "error";
                timeout_event.task_id = task_id;
                timeout_event.error = "Timed out waiting for job completion";
                timeout_event.message = timeout_event.error;
                timeout_event.done = true;
                events.push_back(timeout_event);
                return events;
            }

            std::cout << "Streaming job events for: " << task_id << '\n';
            auto job = findJob(task_id);
            if (!job) {
                TaskEvent error_event;
                error_event.event_type = "failed";
                error_event.task_id = task_id;
                error_event.error = "Job not found";
                error_event.done = true;
                return std::vector<TaskEvent>{error_event};
            }

            while (!job->done) {
                std::this_thread::sleep_for(std::chrono::milliseconds(10));
            }

            if (job->worker.joinable()) {
                job->worker.join();
            }

            std::vector<TaskEvent> events;
            {
                std::lock_guard<std::mutex> lock(jobs_mutex_);
                events = job->events;
                jobs_.erase(task_id);
            }
            return events;
        });
    }

    bool CancelTask(const std::string& task_id) {
        if (task_id.empty()) {
            std::cerr << "Job ID is required" << '\n';
            return false;
        }

        std::cout << "Cancelling job: " << task_id << '\n';

        if (IsTCPAddress(config_.address)) {
            if (!connected_ && !connectInternal()) {
                if (IsConnectionError()) {
                    ScheduleReconnectIfNeeded();
                }
                std::cerr << "Not connected to server" << '\n';
                return false;
            }

            ::croupier::sdk::v1::CancelTaskRequest req;
            req.set_task_id(task_id);

            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                if (!transport_ || !transport_->IsConnected()) {
                    std::cerr << "Not connected to server" << '\n';
                    return false;
                }
                transport_->Call(protocol::MSG_CANCEL_TASK_REQUEST, SerializeMessage(req));
            }

            std::lock_guard<std::mutex> lock(jobs_mutex_);
            auto it = jobs_.find(task_id);
            if (it != jobs_.end()) {
                TaskEvent cancelled_event;
                cancelled_event.event_type = "cancelled";
                cancelled_event.task_id = task_id;
                cancelled_event.message = "Job cancelled";
                cancelled_event.done = true;
                it->second->events.push_back(cancelled_event);
                it->second->cancelled = true;
                it->second->done = true;
            }
            return true;
        }

        auto job = findJob(task_id);
        if (!job || job->done) {
            return false;
        }

        job->cancelled = true;
        TaskEvent cancelled;
        cancelled.event_type = "cancelled";
        cancelled.task_id = task_id;
        cancelled.message = "Job cancelled";
        cancelled.done = true;
        appendTaskEvent(job, cancelled);
        std::cout << "Job cancellation sent: " << task_id << '\n';
        return true;
    }

    void SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema) {
        schemas_[function_id] = schema;
        std::cout << "Set schema for function: " << function_id << '\n';
    }

    void SetReconnectConfig(const ReconnectConfig& config) { reconnect_config_ = config; }

    void SetRetryConfig(const RetryConfig& config) { retry_config_ = config; }

    void Close() {
        // Idempotent close - only execute once
        if (closed_.exchange(true)) {
            return;  // Already closed
        }

        // First, signal all threads to stop
        should_stop_reconnecting_ = true;
        connected_ = false;

        // Detach reconnect thread to avoid deadlock
        // The thread will exit naturally when it checks should_stop_reconnecting_
        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            if (reconnect_thread_.joinable()) {
                reconnect_thread_.detach();
            }
        }

        // Signal all jobs to cancel
        std::vector<std::shared_ptr<LocalJobState>> jobs_to_cancel;
        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            for (const auto& entry : jobs_) {
                jobs_to_cancel.push_back(entry.second);
            }
        }
        for (const auto& job : jobs_to_cancel) {
            job->cancelled = true;
            job->done = true;
            // Detach worker threads instead of joining to avoid deadlock
            if (job->worker.joinable()) {
                job->worker.detach();
            }
        }
        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            jobs_.clear();
        }

        // Close transport - this sets closing_ flag and closes socket
        // This will unblock any pending Call() operations
        {
            std::lock_guard<std::mutex> lock(transport_mutex_);
            if (transport_) {
                transport_->Close();
                transport_.reset();
            }
        }

        schemas_.clear();
        SDK_LOG_INFO("Invoker closed");
    }

    std::shared_ptr<LocalJobState> findJob(const std::string& task_id) {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        auto it = jobs_.find(task_id);
        if (it == jobs_.end()) {
            return nullptr;
        }
        return it->second;
    }

    void appendTaskEvent(const std::shared_ptr<LocalJobState>& job, const TaskEvent& event) {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        if (job->done) {
            return;
        }
        job->events.push_back(event);
        if (event.done) {
            job->done = true;
        }
    }

    // Check if error is a connection error
    bool IsConnectionError() const {
        std::string lower_error = last_error_;
        std::transform(lower_error.begin(), lower_error.end(), lower_error.begin(), ::tolower);

        // Check for common connection error patterns
        return lower_error.find("connection") != std::string::npos ||
               lower_error.find("refused") != std::string::npos || lower_error.find("reset") != std::string::npos ||
               lower_error.find("unreachable") != std::string::npos || lower_error.find("timeout") != std::string::npos;
    }

    // Calculate reconnection delay with exponential backoff and jitter
    int CalculateReconnectDelay() const {
        // Calculate base delay using exponential backoff
        int base_delay = reconnect_config_.initial_delay_ms;
        int exponential_delay =
            static_cast<int>(base_delay * std::pow(reconnect_config_.backoff_multiplier, reconnect_attempts_ - 1));

        // Cap at max delay
        if (exponential_delay > reconnect_config_.max_delay_ms) {
            exponential_delay = reconnect_config_.max_delay_ms;
        }

        // Add jitter to prevent thundering herd
        std::random_device rd;
        // Use additional entropy to avoid narrowing conversion
        std::mt19937 gen(rd() ^ (rd() << 16));
        std::uniform_real_distribution<> dis(-reconnect_config_.jitter_factor, reconnect_config_.jitter_factor);
        double jitter_ratio = dis(gen);

        int jitter = static_cast<int>(exponential_delay * jitter_ratio);
        int final_delay = exponential_delay + jitter;

        if (final_delay < 0) {
            final_delay = 0;
        }

        return final_delay;
    }

    // Schedule reconnection if enabled
    void ScheduleReconnectIfNeeded() {
        // Don't schedule if already closed
        if (closed_ || should_stop_reconnecting_) {
            return;
        }

        if (!reconnect_config_.enabled) {
            return;
        }

        if (is_reconnecting_) {
            return;
        }

        // Check max attempts
        if (reconnect_config_.max_attempts > 0 && reconnect_attempts_ >= reconnect_config_.max_attempts) {
            std::cout << "Max reconnection attempts (" << reconnect_config_.max_attempts << ") reached, giving up"
                      << '\n';
            return;
        }

        is_reconnecting_ = true;
        reconnect_attempts_++;

        int delay = CalculateReconnectDelay();
        std::cout << "Scheduling reconnection attempt " << reconnect_attempts_ << " in " << delay << " ms" << '\n';

        // Stop existing reconnect thread if any (use mutex to protect)
        std::thread old_thread;
        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            if (reconnect_thread_.joinable()) {
                old_thread = std::move(reconnect_thread_);
            }
        }
        if (old_thread.joinable()) {
            old_thread.join();
        }

        // Start reconnection thread
        std::thread new_thread([this, delay]() {
            // Use interruptible sleep with 100ms intervals to check for stop signal
            const int sleep_interval_ms = 100;
            int elapsed = 0;
            while (elapsed < delay && !should_stop_reconnecting_ && !closed_) {
                int remaining = delay - elapsed;
                int sleep_time = (remaining < sleep_interval_ms) ? remaining : sleep_interval_ms;
                std::this_thread::sleep_for(std::chrono::milliseconds(sleep_time));
                elapsed += sleep_time;
            }

            if (should_stop_reconnecting_ || closed_) {
                is_reconnecting_ = false;
                return;
            }

            std::cout << "Reconnecting... (attempt " << reconnect_attempts_ << ")" << '\n';
            if (connectInternal()) {
                std::cout << "Reconnection successful" << '\n';
            } else {
                std::cout << "Reconnection attempt " << reconnect_attempts_ << " failed" << '\n';
                // Schedule next attempt (only if not stopping)
                if (!should_stop_reconnecting_ && !closed_) {
                    ScheduleReconnectIfNeeded();
                }
            }
        });

        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            reconnect_thread_ = std::move(new_thread);
        }
    }

    // Check if error is retryable based on status code
    bool IsRetryableError(int status_code) const {
        for (int code : retry_config_.retryable_status_codes) {
            if (code == status_code) {
                return true;
            }
        }
        return false;
    }

    // Calculate retry delay with exponential backoff and jitter
    int CalculateRetryDelay(int attempt) const {
        // Calculate base delay using exponential backoff
        int base_delay = retry_config_.initial_delay_ms;
        int exponential_delay = static_cast<int>(base_delay * std::pow(retry_config_.backoff_multiplier, attempt));

        // Cap at max delay
        if (exponential_delay > retry_config_.max_delay_ms) {
            exponential_delay = retry_config_.max_delay_ms;
        }

        // Add jitter to prevent thundering herd
        std::random_device rd;
        // Use additional entropy to avoid narrowing conversion
        std::mt19937 gen(rd() ^ (rd() << 16));
        std::uniform_real_distribution<> dis(-retry_config_.jitter_factor, retry_config_.jitter_factor);
        double jitter_ratio = dis(gen);

        int jitter = static_cast<int>(exponential_delay * jitter_ratio);
        int final_delay = exponential_delay + jitter;

        if (final_delay < 0) {
            final_delay = 0;
        }

        return final_delay;
    }
};
#endif
