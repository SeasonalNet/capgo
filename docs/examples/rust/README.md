# Rust

Rust can invoke the CLI with `std::process::Command` and deserialize the JSON envelope with `serde_json`.

```rust
use std::env;
use std::io::{self, Read, Write};
use std::process::{Command, Stdio};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut xml = Vec::new();
    io::stdin().read_to_end(&mut xml)?;
    let command = env::var("CAPGO_BIN").unwrap_or_else(|_| "capgo".to_owned());
    let mut child = Command::new(command)
        .args(["-profile", "nws", "-compact"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;
    child.stdin.take().ok_or("capgo stdin unavailable")?.write_all(&xml)?;
    let output = child.wait_with_output()?;
    if output.status.code() != Some(0) && output.status.code() != Some(1) {
        return Err(String::from_utf8_lossy(&output.stderr).into());
    }

    let document: serde_json::Value = serde_json::from_slice(&output.stdout)?;
    println!("{}", document["alert"]["identifier"]);
    if document["valid"] == false {
        eprintln!("{}", document["issues"]);
        std::process::exit(1);
    }
    Ok(())
}
```

For production code, write the XML to the child process's stdin before waiting if the input is large; the abbreviated example can be adapted with `Child::stdin` for streaming.
