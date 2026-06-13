#include <iostream>
#include <string>
#include <sstream>
#include <cstring>
#include <cstdlib>
#include <csignal>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>
#include <vector>
#include <map>
#include <algorithm>
#include <cmath>
#include <iomanip>

static volatile bool keep_running = true;

void handle_sigint(int)
{
    keep_running = false;
}

// ─── Minimal JSON field extractor (no external deps) ────────────────────────

// Extract a string value for a given key from a JSON object string.
// e.g. json_string_field(body, "side") → "BUY"
static std::string json_string_field(const std::string &json, const std::string &key)
{
    std::string search = "\"" + key + "\"";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return "";
    pos = json.find(':', pos + search.size());
    if (pos == std::string::npos) return "";
    pos = json.find('"', pos + 1);
    if (pos == std::string::npos) return "";
    size_t end = json.find('"', pos + 1);
    if (end == std::string::npos) return "";
    return json.substr(pos + 1, end - pos - 1);
}

// Extract a numeric value for a given key from a JSON object string.
// e.g. json_number_field(body, "price") → 100.25
static double json_number_field(const std::string &json, const std::string &key)
{
    std::string search = "\"" + key + "\"";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return 0.0;
    pos = json.find(':', pos + search.size());
    if (pos == std::string::npos) return 0.0;
    // Skip whitespace
    pos++;
    while (pos < json.size() && (json[pos] == ' ' || json[pos] == '\t')) pos++;
    size_t end = pos;
    while (end < json.size() && (std::isdigit(json[end]) || json[end] == '.' || json[end] == '-' || json[end] == 'e' || json[end] == 'E' || json[end] == '+'))
        end++;
    if (end == pos) return 0.0;
    return std::stod(json.substr(pos, end - pos));
}

static bool json_bool_field(const std::string &json, const std::string &key)
{
    std::string search = "\"" + key + "\"";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return false;
    pos = json.find(':', pos + search.size());
    if (pos == std::string::npos) return false;
    return json.find("true", pos) < json.find(',', pos) || json.find("true", pos) < json.find('}', pos);
}

// ─── Orderbook ──────────────────────────────────────────────────────────────

// Price level: aggregated quantity at a single price.
struct PriceLevel {
    double price;
    int    qty;
};

// The orderbook: bids sorted descending, asks sorted ascending.
// Uses std::map for O(log N) insert/lookup; negative key trick for descending bids.
struct Orderbook {
    // key = -price for bids (so begin() gives highest bid)
    // key = +price for asks (so begin() gives lowest ask)
    std::map<double, int> bids; // key = -price → qty
    std::map<double, int> asks; // key = +price → qty

    void add_order(const std::string &side, double price, int qty)
    {
        if (side == "BUY") {
            // Match against asks (lowest ask first)
            while (qty > 0 && !asks.empty()) {
                auto it = asks.begin();
                double ask_price = it->first;
                if (price < ask_price) break; // No cross
                int match = std::min(qty, it->second);
                qty -= match;
                it->second -= match;
                if (it->second <= 0) asks.erase(it);
            }
            if (qty > 0) {
                bids[-price] += qty; // negative key for descending order
            }
        } else {
            // Match against bids (highest bid first)
            while (qty > 0 && !bids.empty()) {
                auto it = bids.begin();
                double bid_price = -(it->first); // un-negate
                if (price > bid_price) break; // No cross
                int match = std::min(qty, it->second);
                qty -= match;
                it->second -= match;
                if (it->second <= 0) bids.erase(it);
            }
            if (qty > 0) {
                asks[price] += qty;
            }
        }
    }

    void cancel_order(const std::string &side, double price, int qty)
    {
        if (side == "BUY") {
            auto it = bids.find(-price);
            if (it != bids.end()) {
                it->second -= qty;
                if (it->second <= 0) bids.erase(it);
            }
        } else {
            auto it = asks.find(price);
            if (it != asks.end()) {
                it->second -= qty;
                if (it->second <= 0) asks.erase(it);
            }
        }
    }

    double best_bid() const {
        if (bids.empty()) return 0.0;
        return -(bids.begin()->first);
    }

    double best_ask() const {
        if (asks.empty()) return 0.0;
        return asks.begin()->first;
    }
};

// ─── HTTP Server ────────────────────────────────────────────────────────────

int main()
{
    signal(SIGINT, handle_sigint);
    const int port = 8080;

    int server_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (server_fd < 0)
    {
        std::cerr << "socket() failed\n";
        return 1;
    }

    int opt = 1;
    setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    sockaddr_in address;
    std::memset(&address, 0, sizeof(address));
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = INADDR_ANY;
    address.sin_port = htons(port);

    if (bind(server_fd, (struct sockaddr *)&address, sizeof(address)) < 0)
    {
        std::cerr << "bind() failed\n";
        close(server_fd);
        return 1;
    }

    if (listen(server_fd, 1024) < 0)
    {
        std::cerr << "listen() failed\n";
        close(server_fd);
        return 1;
    }

    std::cout << "Orderbook Engine Initialized..." << std::endl;
    std::cout << "Listening on 0.0.0.0:" << port << " - Waiting for orders..." << std::endl;

    Orderbook ob;

    while (keep_running)
    {
        sockaddr_in client_addr;
        socklen_t client_len = sizeof(client_addr);
        int client_fd = accept(server_fd, (struct sockaddr *)&client_addr, &client_len);
        if (client_fd < 0)
        {
            if (!keep_running)
                break;
            std::cerr << "accept() failed\n";
            continue;
        }

        std::string request_data;
        char buffer[4096];
        ssize_t n;

        bool headers_complete = false;
        size_t content_length = 0;
        size_t header_end_pos = 0;

        // 1. Read until we find the end of the HTTP headers (\r\n\r\n)
        while (!headers_complete) {
            n = recv(client_fd, buffer, sizeof(buffer), 0);
            if (n <= 0) break;

            request_data.append(buffer, n);
            header_end_pos = request_data.find("\r\n\r\n");

            if (header_end_pos != std::string::npos) {
                headers_complete = true;

                // Parse Content-Length
                std::string headers = request_data.substr(0, header_end_pos);
                size_t cl_pos = headers.find("Content-Length: ");
                if (cl_pos != std::string::npos) {
                    size_t cl_end = headers.find("\r\n", cl_pos);
                    std::string cl_str = headers.substr(cl_pos + 16, cl_end - (cl_pos + 16));
                    content_length = std::stoul(cl_str);
                }
            }
        }

        // 2. Read remaining body bytes
        if (headers_complete && content_length > 0) {
            size_t body_bytes_read = request_data.length() - (header_end_pos + 4);

            while (body_bytes_read < content_length) {
                n = recv(client_fd, buffer, sizeof(buffer), 0);
                if (n <= 0) break;

                request_data.append(buffer, n);
                body_bytes_read += n;
            }
        }

        // 3. Extract JSON body and process the order
        std::string json_body;
        if (headers_complete && header_end_pos != std::string::npos) {
            json_body = request_data.substr(header_end_pos + 4);
        }

        if (!json_body.empty()) {
            std::string action = json_string_field(json_body, "action");
            std::string side   = json_string_field(json_body, "side");
            double price       = json_number_field(json_body, "price");
            int quantity       = (int)json_number_field(json_body, "quantity");
            bool cancel        = json_bool_field(json_body, "cancel");

            if (action == "CANCEL" || cancel) {
                ob.cancel_order(side, price, quantity);
            } else {
                ob.add_order(side, price, quantity);
            }
        }

        // 4. Build response with actual BBO
        double bb = ob.best_bid();
        double ba = ob.best_ask();

        std::ostringstream body_ss;
        body_ss << std::fixed << std::setprecision(6);
        body_ss << "{\"status\":\"accepted\",\"best_bid\":" << bb
                << ",\"best_ask\":" << ba << "}";
        std::string body = body_ss.str();

        std::ostringstream resp;
        resp << "HTTP/1.1 200 OK\r\n";
        resp << "Content-Type: application/json\r\n";
        resp << "Content-Length: " << body.size() << "\r\n";
        resp << "Connection: close\r\n";
        resp << "\r\n";
        resp << body;

        std::string out = resp.str();
        send(client_fd, out.c_str(), out.size(), MSG_NOSIGNAL);
        close(client_fd);
    }

    close(server_fd);
    std::cout << "Shutting down engine" << std::endl;
    return 0;
}