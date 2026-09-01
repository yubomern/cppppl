// C++ / Boost.Beast web application
// ----------------------------------
// - Boost.Asio + Boost.Beast HTTP server (single-threaded, synchronous per
//   connection, one thread per accepted connection for simplicity/clarity)
// - nlohmann::json for all request/response bodies
// - Plain-text JSON files on disk (data/users.json, data/items.json) as
//   the persistence layer (no external database)
// - Cookie-based session login: POST /api/register, /api/login, /api/logout
// - Session-protected CRUD REST API for an "items" resource:
//     GET    /api/items
//     GET    /api/items/{id}
//     POST   /api/items
//     PUT    /api/items/{id}
//     DELETE /api/items/{id}
// - Serves the static frontend (HTML/CSS/JS) from ./public
//
// Build: see CMakeLists.txt
// Run:   ./webapp_server [port]   (default port 8080)

#include <boost/asio.hpp>
#include <boost/beast.hpp>
#include <nlohmann/json.hpp>

#include "ThreadPool.h"

#include <cstdlib>
#include <fstream>
#include <iostream>
#include <sstream>
#include <thread>

#include "json_store.h"
#include "session_manager.h"
#include "sha256.h"
#ifdef _WIN32
#define POPEN  _popen
#define PCLOSE _pclose
#else
#define POPEN  popen
#define PCLOSE pclose
#endif
namespace asio = boost::asio;
namespace beast = boost::beast;
namespace http = beast::http;
using tcp = asio::ip::tcp;
using json = nlohmann::json;

// ═══════════════════════════════════════════════════════════════════════
//  GLOBAL STATE
// ═══════════════════════════════════════════════════════════════════════
static JsonStore g_users("data/users.json");
static JsonStore g_items("data/items.json");
static SessionManager g_sessions;
static const std::string PUBLIC_DIR = "public";
static const std::string SESSION_COOKIE_NAME = "session_id";

// ═══════════════════════════════════════════════════════════════════════
//  HELPERS
// ═══════════════════════════════════════════════════════════════════════

// Extract a cookie value by name from the raw "Cookie" header.
std::string get_cookie(const std::string& cookie_header, const std::string& name) {
    std::istringstream ss(cookie_header);
    std::string pair;
    while (std::getline(ss, pair, ';')) {
        auto pos = pair.find('=');
        if (pos == std::string::npos) continue;
        std::string key = pair.substr(0, pos);
        // trim leading spaces
        size_t start = key.find_first_not_of(' ');
        if (start != std::string::npos) key = key.substr(start);
        if (key == name) {
            std::string value = pair.substr(pos + 1);
            return value;
        }
    }
    return "";
}

// Returns the logged-in username for a request, or "" if not authenticated.
template <class Request>
std::string current_user(const Request& req) {
    auto it = req.find(http::field::cookie);
    if (it == req.end()) return "";
    std::string token = get_cookie(std::string(it->value()), SESSION_COOKIE_NAME);
    if (token.empty()) return "";
    return g_sessions.username_for(token);
}

std::string mime_type(const std::string& path) {
    auto ext = path.substr(path.find_last_of('.') + 1);
    if (ext == "html") return "text/html; charset=utf-8";
    if (ext == "css") return "text/css; charset=utf-8";
    if (ext == "js") return "application/javascript; charset=utf-8";
    if (ext == "json") return "application/json; charset=utf-8";
    if (ext == "png") return "image/png";
    if (ext == "jpg" || ext == "jpeg") return "image/jpeg";
    if (ext == "svg") return "image/svg+xml";
    if (ext == "ico") return "image/x-icon";
    return "application/octet-stream";
}

// Reads a static file from ./public into a string. Returns false if not found.
bool read_static_file(const std::string& rel_path, std::string& out) {
    std::string full = PUBLIC_DIR + rel_path;
    std::ifstream file(full, std::ios::binary);
    if (!file.good()) return false;
    std::ostringstream ss;
    ss << file.rdbuf();
    out = ss.str();
    return true;
}

json safe_parse(const std::string& body) {
    try {
        return json::parse(body);
    }
    catch (...) {
        return json::object();
    }
}

// ═══════════════════════════════════════════════════════════════════════
//  RESPONSE BUILDERS
// ═══════════════════════════════════════════════════════════════════════
http::response<http::string_body> json_response(
    http::status status, const json& body,
    unsigned version, bool keep_alive,
    const std::string& set_cookie = "") {
    http::response<http::string_body> res{ status, version };
    res.set(http::field::server, "cpp-boost-crud/1.0");
    res.set(http::field::content_type, "application/json; charset=utf-8");
    res.set(http::field::access_control_allow_origin, "*");
    if (!set_cookie.empty())
        res.set(http::field::set_cookie, set_cookie);
    res.keep_alive(keep_alive);
    res.body() = body.dump();
    res.prepare_payload();
    return res;
}

http::response<http::string_body> text_response(
    http::status status, const std::string& body, const std::string& content_type,
    unsigned version, bool keep_alive) {
    http::response<http::string_body> res{ status, version };
    res.set(http::field::server, "cpp-boost-crud/1.0");
    res.set(http::field::content_type, content_type);
    res.keep_alive(keep_alive);
    res.body() = body;
    res.prepare_payload();
    return res;
}

// ═══════════════════════════════════════════════════════════════════════
//  ROUTE HANDLERS
// ═══════════════════════════════════════════════════════════════════════

// --- POST /api/register ---
http::response<http::string_body> handle_register(
    const http::request<http::string_body>& req) {
    json body = safe_parse(req.body());
    std::string username = body.value("username", "");
    std::string password = body.value("password", "");

    if (username.empty() || password.size() < 4) {
        return json_response(http::status::bad_request,
            { {"error", "username required, password must be at least 4 characters"} },
            req.version(), req.keep_alive());
    }

    auto existing = g_users.find_if([&](const json& u) {
        return u.value("username", "") == username;
        });
    if (existing.has_value()) {
        return json_response(http::status::conflict,
            { {"error", "username already taken"} }, req.version(), req.keep_alive());
    }

    json new_user = {
        {"username", username},
        {"password_hash", SHA256::hash(password)}
    };
    json created = g_users.insert(new_user);

    return json_response(http::status::created,
        { {"message", "account created"}, {"username", username} },
        req.version(), req.keep_alive());
}

// --- POST /api/login ---
http::response<http::string_body> handle_login(
    const http::request<http::string_body>& req) {
    json body = safe_parse(req.body());
    std::string username = body.value("username", "");
    std::string password = body.value("password", "");

    auto user = g_users.find_if([&](const json& u) {
        return u.value("username", "") == username;
        });

    if (!user.has_value() ||
        user->value("password_hash", "") != SHA256::hash(password)) {
        return json_response(http::status::unauthorized,
            { {"error", "invalid username or password"} }, req.version(), req.keep_alive());
    }

    std::string token = g_sessions.create_session(username);
    std::string cookie = SESSION_COOKIE_NAME + "=" + token +
        "; Path=/; HttpOnly; SameSite=Lax; Max-Age=43200";

    return json_response(http::status::ok,
        { {"message", "logged in"}, {"username", username} },
        req.version(), req.keep_alive(), cookie);
}

// --- POST /api/logout ---
http::response<http::string_body> handle_logout(
    const http::request<http::string_body>& req) {
    auto it = req.find(http::field::cookie);
    if (it != req.end()) {
        std::string token = get_cookie(std::string(it->value()), SESSION_COOKIE_NAME);
        if (!token.empty()) g_sessions.destroy_session(token);
    }
    // Expire the cookie client-side too.
    std::string cookie = SESSION_COOKIE_NAME + "=; Path=/; HttpOnly; Max-Age=0";
    return json_response(http::status::ok, { {"message", "logged out"} },
        req.version(), req.keep_alive(), cookie);
}

// --- GET /api/session : who am I? ---
http::response<http::string_body> handle_session_check(
    const http::request<http::string_body>& req) {
    std::string user = current_user(req);
    if (user.empty()) {
        return json_response(http::status::ok, { {"authenticated", false} },
            req.version(), req.keep_alive());
    }
    return json_response(http::status::ok,
        { {"authenticated", true}, {"username", user} }, req.version(), req.keep_alive());
}

// --- CRUD: GET /api/items ---
http::response<http::string_body> handle_items_list(
    const http::request<http::string_body>& req) {
    return json_response(http::status::ok, g_items.all(), req.version(), req.keep_alive());
}

// --- CRUD: GET /api/items/{id} ---
http::response<http::string_body> handle_items_get(
    const http::request<http::string_body>& req, int id) {
    auto item = g_items.find_by_id(id);
    if (!item.has_value())
        return json_response(http::status::not_found, { {"error", "item not found"} },
            req.version(), req.keep_alive());
    return json_response(http::status::ok, *item, req.version(), req.keep_alive());
}

// --- CRUD: POST /api/items ---
http::response<http::string_body> handle_items_create(
    const http::request<http::string_body>& req, const std::string& owner) {
    json body = safe_parse(req.body());
    if (!body.contains("title") || body.value("title", "").empty()) {
        return json_response(http::status::bad_request, { {"error", "title is required"} },
            req.version(), req.keep_alive());
    }
    body["owner"] = owner;
    json created = g_items.insert(body);
    return json_response(http::status::created, created, req.version(), req.keep_alive());
}

// --- CRUD: PUT /api/items/{id} ---
http::response<http::string_body> handle_items_update(
    const http::request<http::string_body>& req, int id) {
    json patch = safe_parse(req.body());
    auto updated = g_items.update(id, patch);
    if (!updated.has_value())
        return json_response(http::status::not_found, { {"error", "item not found"} },
            req.version(), req.keep_alive());
    return json_response(http::status::ok, *updated, req.version(), req.keep_alive());
}

// --- CRUD: DELETE /api/items/{id} ---
http::response<http::string_body> handle_items_delete(
    const http::request<http::string_body>& req, int id) {
    bool removed = g_items.remove(id);
    if (!removed)
        return json_response(http::status::not_found, { {"error", "item not found"} },
            req.version(), req.keep_alive());
    return json_response(http::status::ok, { {"message", "item deleted"}, {"id", id} },
        req.version(), req.keep_alive());
}

// ═══════════════════════════════════════════════════════════════════════
//  ROUTER
// ═══════════════════════════════════════════════════════════════════════
http::response<http::string_body> route(const http::request<http::string_body>& req) {
    std::string target(req.target());
    std::string path = target;
    auto qpos = path.find('?');
    if (qpos != std::string::npos) path = path.substr(0, qpos);

    // ---- API routes ----
    if (path == "/api/register" && req.method() == http::verb::post)
        return handle_register(req);

    if (path == "/api/login" && req.method() == http::verb::post)
        return handle_login(req);

    if (path == "/api/logout" && req.method() == http::verb::post)
        return handle_logout(req);

    if (path == "/api/session" && req.method() == http::verb::get)
        return handle_session_check(req);

    if (path == "/api/items" && req.method() == http::verb::get)
        return handle_items_list(req);

    if (path == "/api/items" && req.method() == http::verb::post) {
        std::string user = current_user(req);
        if (user.empty())
            return json_response(http::status::unauthorized,
                { {"error", "login required"} }, req.version(), req.keep_alive());
        return handle_items_create(req, user);
    }

    // /api/items/{id}  -> GET / PUT / DELETE
    if (path.rfind("/api/items/", 0) == 0) {
        std::string id_str = path.substr(std::string("/api/items/").size());
        try {
            int id = std::stoi(id_str);
            if (req.method() == http::verb::get)
                return handle_items_get(req, id);
            if (req.method() == http::verb::put) {
                std::string user = current_user(req);
                if (user.empty())
                    return json_response(http::status::unauthorized,
                        { {"error", "login required"} }, req.version(), req.keep_alive());
                return handle_items_update(req, id);
            }
            if (req.method() == http::verb::delete_) {
                std::string user = current_user(req);
                if (user.empty())
                    return json_response(http::status::unauthorized,
                        { {"error", "login required"} }, req.version(), req.keep_alive());
                return handle_items_delete(req, id);
            }
        }
        catch (...) {
            return json_response(http::status::bad_request, { {"error", "invalid id"} },
                req.version(), req.keep_alive());
        }
    }

    // ---- Static file serving ----
    std::string file_path = path;
    if (file_path == "/") file_path = "/index.html";
    std::string content;
    if (read_static_file(file_path, content)) {
        return text_response(http::status::ok, content, mime_type(file_path),
            req.version(), req.keep_alive());
    }

    // ---- 404 ----
    return json_response(http::status::not_found, { {"error", "not found: " + path} },
        req.version(), req.keep_alive());
}

// ═══════════════════════════════════════════════════════════════════════
//  CONNECTION HANDLING
// ═══════════════════════════════════════════════════════════════════════
void handle_connection(tcp::socket socket) {
    try {
        beast::flat_buffer buffer;
        for (;;) {
            http::request<http::string_body> req;
            beast::error_code ec;
            http::read(socket, buffer, req, ec);
            if (ec == http::error::end_of_stream) break;
            if (ec) break;

            bool keep_alive = req.keep_alive();
            auto res = route(req);
            http::write(socket, res, ec);
            if (ec || !keep_alive) break;
        }
        beast::error_code ec;
        socket.shutdown(tcp::socket::shutdown_send, ec);
    }
    catch (const std::exception& e) {
        std::cerr << "[connection error] " << e.what() << std::endl;
    }
}

std::mutex loggermtx;

class Logger {



private  :

    Logger() {}
public  :

    Logger(const Logger&) = delete;
    Logger& operator =  (const Logger&) = delete;
    static Logger& getInstance() {
        std::lock_guard<std::mutex> lk(loggermtx);
        static Logger ins;
        return ins;
    }
    void info(const std::string& message) {
        std::cout << "[INFO] " << message << std::endl;
    }
};
#include <memory>

class Crash {

public :

    virtual void execute() = 0;
    virtual ~Crash() = default;

};

class SegfaultCrash :public Crash {

public  :

    void execute() override {
        std::cout << "Segmentation fault\n";
    }
};
class DeadlockCrash : public Crash {
public:
    void execute() override {
        std::cout << "Deadlock\n";
    }
};



class CrashFactory {

public  :
    static std::unique_ptr<Crash> create(const std::string& type) {

        if (type == "segfault")
            return std::make_unique<SegfaultCrash>();

        if (type == "deadlock")
            return std::make_unique<DeadlockCrash>();

        return nullptr;
    }
};




void readLargeFile(const std::string& fileName)
{

    std::ifstream file_(fileName);
    if (!file_.is_open()) {
        std::cerr << "errror open file  " << std::endl; 
        return;
    }

    std::string line;

    while(std::getline(file_, line)) {
        std::cout << line << std::endl;

    }
    file_.close();
}

#include <vector>
#include <array>
std::atomic<int > fileLarger = 0;
int readChunk(const std::string& fileName) {
    std::ifstream file_(fileName); 
    const size_t BUFFER_SIZE = 1024 * 1024; // 1MO  

    std::vector<char> buffer(BUFFER_SIZE);

    while (file_) {
        fileLarger++;
        file_.read(buffer.data(), buffer.size());
        std::streamsize size_ = file_.gcount();
        if (size_ > 0) {
            std::cout << "Read "
                << size_
                << " bytes\n";
        }
    }
    return 1;
}


std::string javaExceute(const std::string& JavaPath, int len_)
{
    std::array<char, 1024> buffer;
    std::string result;
    std::unique_ptr<FILE, decltype(&_pclose)>  pipe(_popen(JavaPath.c_str(), "r"), &_pclose );
    if (!pipe) {
        std::cerr << "popen failed\n";
        return "";
    }
    while (fgets(buffer.data(),
        buffer.size(),
        pipe.get())) {

        result += buffer.data();
    }
    return result;
}

#include "Include.h"
int main(int argc, char* argv[]) {
    ThreadPool<int> pool(4);

    ThreadSafePool poolv2(4);

    auto f1 = poolv2.enqueue([] { std::cout << "Task 1\n"; });
    auto f2 = poolv2.enqueue([](int x) { std::cout << "Task 2: " << x << "\n"; }, 42);

    f1.get();
    f2.get();

    std::cout << "All tasks completed.\n";

    pool.enqueue([] {
        std::cout << "Task 1\n";
        Logger::getInstance().info("Thread Pool started");

        Logger::getInstance().info("Failed to open core file");
        });

    pool.enqueue([] {
        std::cout << "Task 2\n";
        auto crash = CrashFactory::create("segfault");
        if (crash)crash->execute();
        });
    try {
        unsigned short port = 8080;
        if (argc > 1) port = static_cast<unsigned short>(std::atoi(argv[1]));

        asio::io_context ioc{ 1 };
        tcp::acceptor acceptor{ ioc, tcp::endpoint(tcp::v4(), port) };

        std::cout << "==============================================\n";
        std::cout << "  C++ Boost CRUD Web App\n";
        std::cout << "  Listening on http://0.0.0.0:" << port << "\n";
        std::cout << "  Static files served from: ./" << PUBLIC_DIR << "\n";
        std::cout << "  Data stored in: ./data/users.json, ./data/items.json\n";
        std::cout << "==============================================\n";

        for (;;) {
            tcp::socket socket{ ioc };
            acceptor.accept(socket);
            std::thread(&handle_connection, std::move(socket)).detach();
        }
    }
    catch (const std::exception& e) {
        std::cerr << "Fatal error: " << e.what() << std::endl;
        return 1;
    }
    return 0;
}
