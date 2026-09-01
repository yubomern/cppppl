#pragma once

#include <queue>
#include <mutex>
#include <condition_variable>
#include <optional>
#include <thread>
#include <atomic>

template<typename T>
class ThreadPool {

private  :


	std::queue<T>  m_pool;
	mutable std::mutex mtxxx;
	std::condition_variable m_cv;


    std::vector<std::thread> workers;
    std::queue<std::function<void()>> tasks;

    std::mutex mutex;
    std::condition_variable condition;
    bool stop = false;



    std::mutex m_mutex;



public: 


	ThreadPool() = default;
    ThreadPool(size_t n) {
        for (size_t i = 0; i < n; ++i) {
            workers.emplace_back([this]() {
                while (true) {
                    std::function<void()> task;

                    {
                        std::unique_lock<std::mutex> lock(mutex);

                        condition.wait(lock, [this]() {
                            return stop || !tasks.empty();
                            });

                        if (stop && tasks.empty())
                            return;

                        task = std::move(tasks.front());
                        tasks.pop();
                    }

                    task();
                }
                });
        }
    }

    void enqueue(std::function<void()> task) {
        {
            std::lock_guard<std::mutex> lock(mutex);
            tasks.push(std::move(task));
        }

        condition.notify_one();
    }


    
    ThreadPool(const ThreadPool&) = delete;
    ThreadPool& operator=(const ThreadPool&) = delete;
    void push(T item) {
        {
            std::lock_guard<std::mutex> lock(m_mutex);
            m_pool.push(std::move(item));
        }
        // Notify one waiting thread that an item is available
        m_cv.notify_one();
    }

    // Blocking pop: Waits until an item is available, then extracts it
    T pop() {
        std::unique_lock<std::mutex> lock(m_mutex);

        // Wait until the pool is not empty
        m_cv.wait(lock, [this]() { return !m_pool.empty(); });

        T item = std::move(m_pool.front());
        m_pool.pop();
        return item;
    }

    std::optional<T> try_pop() {
        std::lock_guard<std::mutex> lock(m_mutex);
        if (m_pool.empty()) {
            return std::nullopt;
        }

        T item = std::move(m_pool.front());
        m_pool.pop();
        return item;
    }
    bool empty() const {
        std::lock_guard<std::mutex> lock(m_mutex);
        return m_pool.empty();
    }




};