#!/usr/bin/env python3
import os
import urllib.error
import urllib.parse
import urllib.request

def build_message(repo, workflow, job, ref, sha, run_url):
    return "\n".join([
        "CI failed on release",
        f"repo: {repo}",
        f"workflow: {workflow}",
        f"job: {job}",
        f"ref: {ref}",
        f"sha: {sha}",
        f"run: {run_url}",
    ])

def main():
    bot_token = os.environ["BOT_TOKEN"]
    chat_id = os.environ["CHAT_ID"]
    message = build_message(
        os.environ["REPO"],
        os.environ["WORKFLOW"],
        os.environ["JOB"],
        os.environ["REF"],
        os.environ["SHA"],
        os.environ["RUN_URL"],
    )
    payload = urllib.parse.urlencode(
        {
            "chat_id": chat_id,
            "text": message,
            "disable_web_page_preview": "true",
        }
    ).encode("utf-8")
    url = f"{os.environ['API_BASE'].rstrip('/')}/bot{bot_token}/sendMessage"
    req = urllib.request.Request(url, data=payload, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            resp.read()
    except urllib.error.URLError as exc:
        print(f"telegram alert failed: {exc}")
        raise SystemExit(1)

main()
