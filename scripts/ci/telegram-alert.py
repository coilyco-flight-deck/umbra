#!/usr/bin/env python3
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

def build_message():
    return "\n".join([
        "CI failed on main",
        f"repo: {os.environ['REPO']}",
        f"workflow: {os.environ['WORKFLOW']}",
        f"job: {os.environ['JOB']}",
        f"ref: {os.environ['REF']}",
        f"sha: {os.environ['SHA']}",
        f"run: {os.environ['RUN_URL']}",
    ])

def main():
    bot_token = os.environ.get("BOT_TOKEN", "")
    chat_id = os.environ.get("CHAT_ID", "")
    if not bot_token or not chat_id:
        print("telegram alert missing required secret", file=sys.stderr)
        return 2
    proxy_url = os.environ.get("FORGEJO_EGRESS_PROXY", "").strip()
    payload = urllib.parse.urlencode(
        {
            "chat_id": chat_id,
            "text": build_message(),
            "disable_web_page_preview": "true",
        }
    ).encode("utf-8")
    api_base = os.environ.get("API_BASE", "https://api.telegram.org").rstrip("/")
    request = urllib.request.Request(
        f"{api_base}/bot{bot_token}/sendMessage",
        data=payload,
        method="POST",
    )
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({"https": proxy_url})
        if proxy_url
        else urllib.request.ProxyHandler()
    )
    try:
        with opener.open(request, timeout=15) as response:
            response.read()
    except urllib.error.URLError as exc:
        print(f"telegram alert failed ({type(exc).__name__})", file=sys.stderr)
        return 1
    return 0

raise SystemExit(main())
