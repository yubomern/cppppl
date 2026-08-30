// Simple file-backed JSON storage. Each "table" is a JSON array persisted
// to a plain text .json file on disk. Reads/writes take a process-wide
// mutex per store instance so concurrent HTTP requests don't corrupt files.
#pragma once
#include <nlohmann/json.hpp>
#include <fstream>
#include <mutex>
#include <string>
#include <optional>
#include <algorithm>

using json = nlohmann::json;

class JsonStore {
public:
    explicit JsonStore(std::string path) : path_(std::move(path)) {
        std::lock_guard<std::mutex> lock(mutex_);
        std::ifstream in(path_);
        if (!in.good()) {
            // File doesn't exist yet: create it with an empty array.
            json empty = json::array();
            std::ofstream out(path_);
            out << empty.dump(2);
        }
    }

    // Load the whole array from disk.
    json all() {
        std::lock_guard<std::mutex> lock(mutex_);
        return read_locked();
    }

    // Overwrite the whole array on disk.
    void save_all(const json& arr) {
        std::lock_guard<std::mutex> lock(mutex_);
        write_locked(arr);
    }

    // Append one record, auto-assigning an incrementing integer "id".
    json insert(json record) {
        std::lock_guard<std::mutex> lock(mutex_);
        json arr = read_locked();
        int next_id = 1;
        for (auto& item : arr)
            if (item.contains("id") && item["id"].get<int>() >= next_id)
                next_id = item["id"].get<int>() + 1;
        record["id"] = next_id;
        arr.push_back(record);
        write_locked(arr);
        return record;
    }

    // Find one record by integer id. Returns std::nullopt if not found.
    std::optional<json> find_by_id(int id) {
        std::lock_guard<std::mutex> lock(mutex_);
        json arr = read_locked();
        for (auto& item : arr)
            if (item.contains("id") && item["id"].get<int>() == id)
                return item;
        return std::nullopt;
    }

    // Update fields of an existing record (merge). Returns updated record,
    // or std::nullopt if the id didn't exist.
    std::optional<json> update(int id, const json& patch) {
        std::lock_guard<std::mutex> lock(mutex_);
        json arr = read_locked();
        for (auto& item : arr) {
            if (item.contains("id") && item["id"].get<int>() == id) {
                for (auto it = patch.begin(); it != patch.end(); ++it)
                    if (it.key() != "id")
                        item[it.key()] = it.value();
                write_locked(arr);
                return item;
            }
        }
        return std::nullopt;
    }

    // Remove a record by id. Returns true if something was removed.
    bool remove(int id) {
        std::lock_guard<std::mutex> lock(mutex_);
        json arr = read_locked();
        auto before = arr.size();
        json filtered = json::array();
        for (auto& item : arr)
            if (!(item.contains("id") && item["id"].get<int>() == id))
                filtered.push_back(item);
        write_locked(filtered);
        return filtered.size() != before;
    }

    // Generic search helper: first record matching predicate, or nullopt.
    template <typename Pred>
    std::optional<json> find_if(Pred pred) {
        std::lock_guard<std::mutex> lock(mutex_);
        json arr = read_locked();
        for (auto& item : arr)
            if (pred(item))
                return item;
        return std::nullopt;
    }

private:
    json read_locked() {
        std::ifstream in(path_);
        if (!in.good()) return json::array();
        json data;
        try {
            in >> data;
        }
        catch (...) {
            return json::array();
        }
        if (!data.is_array()) return json::array();
        return data;
    }

    void write_locked(const json& arr) {
        std::ofstream out(path_, std::ios::trunc);
        out << arr.dump(2);
    }

    std::string path_;
    std::mutex mutex_;
};
