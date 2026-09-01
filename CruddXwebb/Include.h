#pragma once
#define _SILENCE_ALL_CXX17_DEPRECATION_WARNINGS
#define _SILENCE_CXX17_RESULT_OF_DEPRECATION_WARNING
#include <iostream>
#include <vector>
#include <thread>
#include <queue>
#include <functional>
#include <condition_variable>
#include <mutex>
#include <future>
#include <atomic>



class ThreadSafePool {


   private  :


       std::vector<std::thread> workers;
       std::queue<std::function<void()>> tasks;
       std::mutex mtx;
       std::condition_variable cv_;
       std::atomic<bool> stop{ false };
    public  :
        explicit ThreadSafePool(size_t nth) :stop(false) {
            for (size_t i = 0; i < nth; i++) {
                workers.emplace_back([this] {
                    while (true) {
                        std::function<void()> task;
                        {
                            std::unique_lock<std::mutex> lk(mtx);
                            cv_.wait(lk, [this] {
                                return stop || !tasks.empty();
                                });

                            if (stop && tasks.empty())return;
                            task = std::move(tasks.front()); 
                            tasks.pop();




                        }
                        task();
                    }

                    });
            }
        }
        /*template<class F, class... Args>
        auto enqueue(F&& f, Args&&... args)
            -> std::future<typename std::result_of<F(Args...)>::type> {

            using return_type = typename std::result_of<F(Args...)>::type;
            auto task = std::make_shared<std::packaged_task<return_type()>>(
                std::bind(std::forward<F>(f), std::forward<Args>(args)...)
            );

            std::future<return_type> res = task->get_future();
            {
                std::unique_lock<std::mutex> lock(mtx);
                if (stop) throw std::runtime_error("enqueue on stopped ThreadPool");
                tasks.emplace([task]() { (*task)(); });
            }
            cv_.notify_one();
            return res;
        }
        */
        template<class F, class... Args>
        auto enqueue(F&& f, Args&&... args)
            -> std::future<std::invoke_result_t<F, Args...>> {

            using return_type = std::invoke_result_t<F, Args...>;
            auto task = std::make_shared<std::packaged_task<return_type()>>(
                std::bind(std::forward<F>(f), std::forward<Args>(args)...)
            );

            std::future<return_type> res = task->get_future();
            {
                std::unique_lock<std::mutex> lock(mtx);
                if (stop) throw std::runtime_error("enqueue on stopped ThreadPool");
                tasks.emplace([task]() { (*task)(); });
            }
            cv_.notify_one();
            return res;
        }

        ~ThreadSafePool() {
            stop = true;
            cv_.notify_all();
            for (std::thread& worker : workers) {
                if (worker.joinable()) worker.join();
            }
        }


};