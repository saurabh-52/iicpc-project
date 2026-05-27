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

static volatile bool keep_running = true;

void handle_sigint(int)
{
    keep_running = false;
}

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

    if (listen(server_fd, 1024) < 0) // Increased backlog for stress tests
    {
        std::cerr << "listen() failed\n";
        close(server_fd);
        return 1;
    }

    std::cout << "Orderbook Engine Initialized..." << std::endl;
    std::cout << "Listening on 0.0.0.0:" << port << " - Waiting for orders..." << std::endl;

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
            if (n <= 0) break; // Client disconnected or error
            
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

        // 2. Calculate how much body data we've already read, and read the rest
        if (headers_complete && content_length > 0) {
            size_t body_bytes_read = request_data.length() - (header_end_pos + 4);
            
            while (body_bytes_read < content_length) {
                n = recv(client_fd, buffer, sizeof(buffer), 0);
                if (n <= 0) break; // Client disconnected or error
                
                request_data.append(buffer, n);
                body_bytes_read += n;
            }
        }

        // 3. Now that the request is fully consumed, send the response
        std::string body = "{\"status\":\"ok\"}";
        std::ostringstream resp;
        resp << "HTTP/1.1 200 OK\r\n";
        resp << "Content-Type: application/json\r\n";
        resp << "Content-Length: " << body.size() << "\r\n";
        resp << "Connection: close\r\n";
        resp << "\r\n";
        resp << body;

        std::string out = resp.str();
        // Use MSG_NOSIGNAL to prevent SIGPIPE if client already disconnected
        send(client_fd, out.c_str(), out.size(), MSG_NOSIGNAL); 
        close(client_fd);
    }

    close(server_fd);
    std::cout << "Shutting down engine" << std::endl;
    return 0;
}