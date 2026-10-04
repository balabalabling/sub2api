"""Verify old-image recovery after deployment. Network requests require --send."""
import argparse
import copy
import json
import os
from pathlib import Path
import sys
import tomllib
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("Recovery requests must not redirect credentials")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--payload", type=Path, required=True)
    parser.add_argument("--codex-home", type=Path, default=Path.home() / ".codex")
    parser.add_argument("--send", action="store_true")
    args = parser.parse_args()
    payload = json.loads(args.payload.read_text(encoding="utf-8"))
    images = [i for i in payload.get("input", []) if isinstance(i, dict) and i.get("type") == "image_generation_call"]
    if not images or any(not str(i.get("id", "")).startswith("ig_") or not isinstance(i.get("result"), str) or not i["result"] for i in images):
        raise ValueError("Complete image-generation items are required")
    payload.update(store=False, stream=True, tools=[], tool_choice="none")
    payload["instructions"] = "Do not generate images or call tools. Reply with exactly OK."
    print(json.dumps({"payload": str(args.payload.resolve()), "image_count": len(images), "will_send": args.send}))
    if not args.send:
        return 0
    config = tomllib.loads((args.codex_home / "config.toml").read_text(encoding="utf-8-sig"))
    provider = config["model_providers"][config["model_provider"]]
    token = provider.get("experimental_bearer_token") or os.environ.get(provider.get("env_key", ""))
    if not token:
        raise ValueError("Configure the original provider/API Key first")
    base_url = provider["base_url"].rstrip("/")
    if urllib.parse.urlparse(base_url).scheme != "https":
        raise ValueError("Use the configured HTTPS gateway")
    opener = urllib.request.build_opener(NoRedirect())
    verification = copy.deepcopy(payload)
    for item in verification["input"]:
        if isinstance(item, dict) and item.get("type") == "image_generation_call":
            item.pop("result", None)
    for stage, body in (("seed", payload), ("verify_without_result", verification)):
        request = urllib.request.Request(base_url + "/responses", data=json.dumps(body).encode(), headers={"Authorization": "Bearer " + token, "Content-Type": "application/json", "User-Agent": "sub2api-image-replay-recovery"})
        try:
            with opener.open(request, timeout=90) as response:
                status = response.status
                result = response.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as error:
            code = error.code
            error.close()
            raise RuntimeError(f"{stage}: HTTP {code}") from None
        events = []
        for line in result.splitlines():
            if line.startswith("data:"):
                try:
                    events.append(json.loads(line[5:].strip()).get("type"))
                except json.JSONDecodeError:
                    continue
        if status != 200 or "response.completed" not in events or any(t in events for t in ("error", "response.failed")):
            raise RuntimeError(f"{stage}: no successful completed response")
        print(json.dumps({"stage": stage, "status": status, "completed": True}))
    print("Cache verified. Continue with the original API Key within 30 minutes; replicas do not share this cache.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, RuntimeError, KeyError) as error:
        print(f"Recovery stopped: {error}", file=sys.stderr)
        raise SystemExit(1)
