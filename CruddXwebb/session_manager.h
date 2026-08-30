#pragma once

// In-memory session store: maps a random session token (sent to the
// browser as a cookie) to a username. Sessions are lost on server
// restart by design (this is a demo auth layer, not production-grade).
#pragma once
#include <chrono>
#include <mutex>
#include <random>
#include <string>
#include <unordered_map>

class SessionManager {
public:
    struct SessionData {
        std::string username;
        std::chrono::steady_clock::time_point created_at;
    };

    // Create a new session for a username, return the session token.
    std::string create_session(const std::string& username) {
        std::lock_guard<std::mutex> lock(mutex_);
        std::string token = generate_token();
        sessions_[token] = SessionData{ username, std::chrono::steady_clock::now() };
        return token;
    }

    // Look up the username for a session token. Empty optional-like
    // string ("") means "not found / expired".
    std::string username_for(const std::string& token) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = sessions_.find(token);
        if (it == sessions_.end()) return "";

        // Expire sessions after 2 hours of inactivity is NOT tracked here
        // for simplicity (no sliding expiry); we only cap total lifetime.
        auto age = std::chrono::steady_clock::now() - it->second.created_at;
        if (age > std::chrono::hours(12)) {
            sessions_.erase(it);
            return "";
        }
        return it->second.username;
    }

    void destroy_session(const std::string& token) {
        std::lock_guard<std::mutex> lock(mutex_);
        sessions_.erase(token);
    }

private:
    std::string generate_token() {
        static const char charset[] =
            "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
        std::random_device rd;
        std::mt19937_64 gen(rd());
        std::uniform_int_distribution<> dist(0, sizeof(charset) - 2);
        std::string token;
        token.reserve(48);
        for (int i = 0; i < 48; ++i)
            token += charset[dist(gen)];
        return token;
    }

    std::unordered_map<std::string, SessionData> sessions_;
    std::mutex mutex_;
};
