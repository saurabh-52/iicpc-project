use std::io::{Read, Write};
use std::net::TcpListener;

fn main() {
    println!("Orderbook Engine Initialized...");
    println!("Listening on 0.0.0.0:8080 - Waiting for orders...");

    let listener = TcpListener::bind("0.0.0.0:8080").expect("Failed to bind to 8080");

    for stream in listener.incoming() {
        match stream {
            Ok(mut stream) => {
                let mut buffer = [0; 1024];
                // Read the request so the client doesn't get connection reset
                let _ = stream.read(&mut buffer);
                
                let response = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: 15\r\n\r\n{\"status\":\"ok\"}";
                let _ = stream.write_all(response.as_bytes());
            }
            Err(_) => {}
        }
    }
}
