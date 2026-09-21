# Python

The Python integration treats `capgo` as a subprocess and decodes its JSON stdout. Set `CAPGO_BIN` to the built command, or leave it as `capgo` when it is on `PATH`.

```python
import json
import os
import subprocess
import sys


def validate_cap(xml_bytes: bytes, profile: str = "nws") -> dict:
    command = os.environ.get("CAPGO_BIN", "capgo")
    result = subprocess.run(
        [command, "-profile", profile, "-compact"],
        input=xml_bytes,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if not result.stdout:
        raise RuntimeError(result.stderr.decode("utf-8", errors="replace"))
    document = json.loads(result.stdout)
    if result.returncode not in (0, 1):
        raise RuntimeError(result.stderr.decode("utf-8", errors="replace"))
    return document


document = validate_cap(sys.stdin.buffer.read(), "nws")
for action in document["actions"]:
    print(action["event"], action["language"])
if not document["valid"]:
    for issue in document["issues"]:
        print(issue["rule"], issue["path"], issue["message"], file=sys.stderr)
    raise SystemExit(1)
```
