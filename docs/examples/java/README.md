# Java

Java can use `ProcessBuilder` and UTF-8 streams to integrate with the CLI without a CAP-specific Java dependency.

```java
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;

public final class CapgoExample {
    public static void main(String[] args) throws Exception {
        String command = System.getenv().getOrDefault("CAPGO_BIN", "capgo");
        byte[] xml = Files.readAllBytes(Path.of("alert.xml"));
        Process process = new ProcessBuilder(command, "-profile", "nws", "-compact")
                .redirectErrorStream(false)
                .start();
        process.getOutputStream().write(xml);
        process.getOutputStream().close();

        String stdout = new String(process.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
        String stderr = new String(process.getErrorStream().readAllBytes(), StandardCharsets.UTF_8);
        int status = process.waitFor();
        if (status != 0 && status != 1) {
            throw new IOException(stderr);
        }

        Map<String, Object> document = new ObjectMapper().readValue(
                stdout, new TypeReference<Map<String, Object>>() {});
        System.out.println(document.get("alert"));
        if (Boolean.FALSE.equals(document.get("valid"))) {
            System.err.println(document.get("issues"));
            System.exit(1);
        }
    }
}
```

The example uses Jackson only for JSON decoding; the CAP parsing and validation remain in `capgo`.
