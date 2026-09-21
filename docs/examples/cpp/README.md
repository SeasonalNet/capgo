# C++

The C++ standard library does not define a portable subprocess API. On POSIX
systems, an application can use `fork`, `exec`, and pipes to send XML to
`capgo` and parse the JSON envelope with a library such as
[`nlohmann/json`](https://github.com/nlohmann/json).

This example leaves `capgo`'s standard error attached to the parent process,
so validation diagnostics remain visible there while JSON stays on standard
output. It uses `CAPGO_BIN` when set and otherwise searches `PATH` for
`capgo`.

```cpp
#include <cstdlib>
#include <iostream>
#include <iterator>
#include <stdexcept>
#include <string>

#include <cerrno>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

#include <nlohmann/json.hpp>

namespace {

void write_all(int fd, const std::string& data) {
    std::size_t offset = 0;
    while (offset < data.size()) {
        const ssize_t written = write(fd, data.data() + offset, data.size() - offset);
        if (written < 0) {
            if (errno == EINTR) continue;
            throw std::runtime_error("writing CAP XML failed");
        }
        offset += static_cast<std::size_t>(written);
    }
}

std::string read_all(int fd) {
    std::string data;
    char buffer[8192];
    for (;;) {
        const ssize_t count = read(fd, buffer, sizeof(buffer));
        if (count == 0) break;
        if (count < 0) {
            if (errno == EINTR) continue;
            throw std::runtime_error("reading capgo output failed");
        }
        data.append(buffer, static_cast<std::size_t>(count));
    }
    return data;
}

} // namespace

int main() {
    const char* configured = std::getenv("CAPGO_BIN");
    const std::string command = configured == nullptr ? "capgo" : configured;
    const std::string xml((std::istreambuf_iterator<char>(std::cin)), {});

    int input_pipe[2];
    int output_pipe[2];
    if (pipe(input_pipe) != 0 || pipe(output_pipe) != 0) {
        throw std::runtime_error("creating capgo pipes failed");
    }

    const pid_t child = fork();
    if (child < 0) throw std::runtime_error("forking capgo failed");
    if (child == 0) {
        dup2(input_pipe[0], STDIN_FILENO);
        dup2(output_pipe[1], STDOUT_FILENO);
        close(input_pipe[0]);
        close(input_pipe[1]);
        close(output_pipe[0]);
        close(output_pipe[1]);
        execlp(command.c_str(), command.c_str(), "-profile", "nws", "-compact", nullptr);
        _exit(127);
    }

    close(input_pipe[0]);
    close(output_pipe[1]);
    write_all(input_pipe[1], xml);
    close(input_pipe[1]);
    const std::string stdout_json = read_all(output_pipe[0]);
    close(output_pipe[0]);

    int status = 0;
    if (waitpid(child, &status, 0) < 0) throw std::runtime_error("waiting for capgo failed");
    if (!WIFEXITED(status) || (WEXITSTATUS(status) != 0 && WEXITSTATUS(status) != 1)) {
        throw std::runtime_error("capgo failed to decode the message");
    }
    if (stdout_json.empty()) {
        throw std::runtime_error("capgo produced no JSON document");
    }

    const auto document = nlohmann::json::parse(stdout_json);
    std::cout << document.at("alert").at("identifier") << '\n';
    if (!document.at("valid").get<bool>()) {
        std::cerr << document.at("issues").dump(2) << '\n';
        return 1;
    }
}
```

Compile with C++17 and the header-only JSON dependency, for example:

```sh
c++ -std=c++17 -O2 -Wall -Wextra -pedantic -I/path/to/json/include capgo_example.cpp -o capgo_example
CAPGO_BIN="$PWD/bin/capgo" ./capgo_example < alert.xml
```

This example uses POSIX process APIs. A Windows application should use its
native process API or a cross-platform process library such as Boost.Process.
For very large inputs, write and read the child pipes concurrently so neither
pipe can fill while the other side is blocked.
