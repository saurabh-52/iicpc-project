use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread;

fn handle_client(mut stream: TcpStream) {
    let mut buffer = [0; 4096];
    let mut request_data = Vec::new();

    let mut headers_complete = false;
    let mut content_length = 0;
    let mut header_end_pos = 0;

    // Read headers
    while !headers_complete {
        match stream.read(&mut buffer) {
            Ok(0) => return,
            Ok(n) => {
                request_data.extend_from_slice(&buffer[..n]);
                if let Some(pos) = request_data.windows(4).position(|window| window == b"\r\n\r\n") {
                    headers_complete = true;
                    header_end_pos = pos;
                    
                    let headers = String::from_utf8_lossy(&request_data[..pos]);
                    if let Some(cl_idx) = headers.find("Content-Length: ") {
                        let cl_start = cl_idx + 16;
                        if let Some(cl_end) = headers[cl_start..].find("\r\n") {
                            if let Ok(cl) = headers[cl_start..cl_start + cl_end].parse::<usize>() {
                                content_length = cl;
                            }
                        }
                    }
                }
            }
            Err(_) => return,
        }
    }

    // Read remaining body if any
    if headers_complete && content_length > 0 {
        let mut body_bytes_read = request_data.len() - (header_end_pos + 4);
        while body_bytes_read < content_length {
            match stream.read(&mut buffer) {
                Ok(0) => break,
                Ok(n) => {
                    body_bytes_read += n;
                }
                Err(_) => break,
            }
        }
    }

    // Send HTTP response
    let body = "{\"status\":\"ok\",\"best_bid\":0.0,\"best_ask\":0.0}";
    let response = format!(
        "HTTP/1.1 200 OK\r\n\
         Content-Type: application/json\r\n\
         Content-Length: {}\r\n\
         Connection: close\r\n\
         \r\n\
         {}",
        body.len(),
        body
    );

    let _ = stream.write_all(response.as_bytes());
}

fn main() {
    let port = 8080;
    let listener = match TcpListener::bind(format!("0.0.0.0:{}", port)) {
        Ok(l) => l,
        Err(e) => {
            eprintln!("Failed to bind to port {}: {}", port, e);
            std::process::exit(1);
        }
    };
    
    println!("Orderbook Engine Initialized...");
    println!("Listening on 0.0.0.0:{} - Waiting for orders...", port);

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                thread::spawn(move || {
                    handle_client(stream);
                });
            }
            Err(e) => {
                eprintln!("Failed to accept connection: {}", e);
            }
        }
    }
}
