#!/usr/bin/env python3
"""Configure a dedicated bot without putting its credential in argv or output."""

import getpass
import json
import os
from pathlib import Path
import re
import secrets
import stat
import sys
import tempfile
import time
import urllib.error
import urllib.request


class SetupError(Exception):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def api(token, method, payload):
    request = urllib.request.Request(
        "https://api.telegram.org/bot" + token + "/" + method,
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    # Exception text can contain the token-bearing URL. Only fixed diagnostics
    # leave this helper; responses and request URLs are never printed.
    try:
        with urllib.request.build_opener(NoRedirect).open(request, timeout=15) as response:
            data = json.loads(response.read(2 * 1024 * 1024))
        if not data.get("ok"):
            raise SetupError("Telegram rejected the request. Check the dedicated bot token and its webhook settings.")
        return data["result"]
    except (urllib.error.URLError, ValueError, KeyError, TimeoutError, OSError):
        raise SetupError("Telegram could not be reached or rejected the request. Check the token, network and webhook settings.") from None


def load_config(path):
    parent = path.parent.stat()
    if parent.st_uid != os.getuid() or parent.st_mode & 0o022:
        raise SetupError("Configuration directory must be owned by you and not writable by others.")
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return "", None
    with os.fdopen(fd, "r", encoding="utf-8") as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) & 0o077:
            raise SetupError("Existing configuration must be a regular file owned by you with mode 0600.")
        return stream.read(), (info.st_dev, info.st_ino, info.st_mtime_ns, info.st_size)


def save_config(path, existing, identity, token, operator):
    lines = [line for line in existing.splitlines() if not re.match(r"\s*(?:export\s+)?TWOPOD_TELEGRAM_(?:BOT_TOKEN|OPERATOR_ID)\s*=", line)]
    lines += ["TWOPOD_TELEGRAM_BOT_TOKEN=" + token, "TWOPOD_TELEGRAM_OPERATOR_ID=" + str(operator)]
    fd, name = tempfile.mkstemp(prefix=".telegram-setup-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            stream.write("\n".join(lines) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
        try:
            info = path.lstat()
            current = (info.st_dev, info.st_ino, info.st_mtime_ns, info.st_size)
        except FileNotFoundError:
            current = None
        if current != identity:
            raise SetupError("Configuration changed during setup. Run this helper again.")
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def main():
    if len(sys.argv) > 2:
        raise SetupError("Usage: python3 scripts/configure-telegram.py [protected-env-file]")
    if not sys.stdin.isatty():
        raise SetupError("Run interactively in a terminal so the token can be entered without echo.")
    path = Path(sys.argv[1] if len(sys.argv) == 2 else ".env").absolute()
    existing, identity = load_config(path)
    print("Use a dedicated bot created through @BotFather /newbot. Stop 2pod before setup.")
    token = getpass.getpass("Dedicated bot token (hidden): ").strip()
    if not re.fullmatch(r"[0-9]+:[A-Za-z0-9_-]+", token):
        raise SetupError("Invalid bot token format.")
    me = api(token, "getMe", {})
    username = me.get("username", "")
    if not me.get("is_bot") or not re.fullmatch(r"[A-Za-z0-9_]+", username):
        raise SetupError("Token did not identify a valid bot.")
    nonce = secrets.token_urlsafe(18)
    challenge = "/start " + nonce
    print("Open @" + username + " in your own Telegram account's private chat.")
    print("Send exactly: " + challenge)
    print("Waiting up to two minutes. No private messages will be printed.")
    offset = 0
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        for update in api(token, "getUpdates", {"offset": offset, "timeout": 5, "allowed_updates": ["message"]}):
            offset = max(offset, update.get("update_id", 0) + 1)
            message = update.get("message", {})
            sender = message.get("from", {}).get("id")
            chat = message.get("chat", {})
            if message.get("text") == challenge and chat.get("type") == "private" and isinstance(sender, int) and sender > 0 and sender == chat.get("id"):
                save_config(path, existing, identity, token, sender)
                print("Saved bot credentials and operator ID to protected configuration: " + str(path))
                print("Restart 2pod to activate Telegram submission. Live delivery still requires a real submission check.")
                return
    raise SetupError("No matching private-chat challenge arrived. Configuration was not changed.")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("Setup cancelled; configuration was not changed.", file=sys.stderr)
        sys.exit(1)
    except (SetupError, OSError, UnicodeError) as error:
        # Filesystem error strings may contain supplied paths; keep them out of
        # output just as Telegram exception strings are kept out above.
        print(str(error) if isinstance(error, SetupError) else "Could not safely read or write configuration.", file=sys.stderr)
        sys.exit(1)
