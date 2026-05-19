use std::io; // Import standard input/output library

fn main() {
    println!("Enter your name:");

    // Create a mutable String to store input
    let mut name = String::new();

    // Read input from the user
    match io::stdin().read_line(&mut name) {
        Ok(_) => {
            // Trim newline characters and print greeting
            let trimmed_name = name.trim();
            if trimmed_name.is_empty() {
                println!("You didn't enter a name!");
            } else {
                println!("Hello, {}!", trimmed_name);
            }
        }
        Err(e) => {
            // Handle input error
            println!("Failed to read input: {}", e);
        }
    }
}
