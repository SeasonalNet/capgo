# C# / .NET

Use `System.Diagnostics.Process` with redirected standard streams and `System.Text.Json` for the result.

```csharp
using System.Diagnostics;
using System.Text.Json;

var command = Environment.GetEnvironmentVariable("CAPGO_BIN") ?? "capgo";
using var input = new StreamReader(Console.OpenStandardInput());
var xml = await input.ReadToEndAsync();
using var process = Process.Start(new ProcessStartInfo {
    FileName = command,
    Arguments = "-profile nws -compact",
    RedirectStandardInput = true,
    RedirectStandardOutput = true,
    RedirectStandardError = true,
    UseShellExecute = false,
});
if (process is null) throw new InvalidOperationException("could not start capgo");

await process.StandardInput.WriteAsync(xml);
process.StandardInput.Close();
var stdout = await process.StandardOutput.ReadToEndAsync();
var stderr = await process.StandardError.ReadToEndAsync();
await process.WaitForExitAsync();
if (process.ExitCode is not (0 or 1)) throw new Exception(stderr);

using var document = JsonDocument.Parse(stdout);
Console.WriteLine(document.RootElement.GetProperty("alert").GetProperty("identifier"));
if (!document.RootElement.GetProperty("valid").GetBoolean()) {
    Console.Error.WriteLine(document.RootElement.GetProperty("issues"));
    Environment.ExitCode = 1;
}
```
