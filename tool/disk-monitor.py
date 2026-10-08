#!/usr/bin/env python3
"""Local disk checks with delivery through Celestia's configured WeCom targets."""

import argparse
import datetime
import json
import os
from pathlib import Path
import plistlib
import socket
import subprocess
import sys
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
LABEL = "com.celestia.disk-monitor"
PLIST = Path.home() / "Library/LaunchAgents" / (LABEL + ".plist")
DOMAIN = "gui/" + str(os.getuid())


def disk_usage(path):
    result = subprocess.run(
        ["/bin/df", "-kP", path], capture_output=True, text=True,
        check=True, timeout=10, env={**os.environ, "LC_ALL": "C"},
    )
    return parse_usage(result.stdout)


def parse_usage(output):
    rows = output.strip().splitlines()
    if len(rows) != 2:
        raise ValueError("df must return exactly one filesystem")
    columns = rows[1].split(None, 5)
    if len(columns) != 6:
        raise ValueError("invalid df output")
    used, available = int(columns[2]), int(columns[3])
    if used < 0 or available < 0 or used + available <= 0:
        raise ValueError("invalid filesystem sizes")
    return {
        "filesystem": columns[0], "mount": columns[5],
        "used_gib": used / 1024**2, "available_gib": available / 1024**2,
        "used_percent": 100 * used / (used + available),
    }


def exceeds_threshold(usage, threshold):
    return usage["used_percent"] > threshold


def api_request(base_url, route, payload=None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        base_url.rstrip("/") + route, data=data,
        headers={"Content-Type": "application/json"},
    )
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=30) as response:
        return json.load(response)


def validate_recipient(args):
    snapshot = api_request(args.gateway_url, "/api/v1/agent")
    if not any(
        user.get("id") == args.to_user and user.get("enabled")
        for user in snapshot.get("push", {}).get("users", [])
    ):
        raise ValueError("WeCom target must be a configured enabled recipient")


def check(args):
    usage = disk_usage(args.path)
    over = exceeds_threshold(usage, args.threshold)
    timestamp = datetime.datetime.now().astimezone().isoformat(timespec="seconds")
    print(json.dumps({"time": timestamp, **usage, "threshold": args.threshold,
                      "over_threshold": over, "dry_run": args.dry_run}, ensure_ascii=False), flush=True)
    if not over or args.dry_run:
        return
    message = (
        "Celestia 磁盘空间告警\n"
        f"主机：{socket.gethostname()}\n磁盘：{usage['mount']}\n"
        f"使用率：{usage['used_percent']:.2f}%（阈值 {args.threshold:g}%）\n"
        f"已用：{usage['used_gib']:.1f} GiB\n"
        f"剩余：{usage['available_gib']:.1f} GiB\n"
        f"检查时间：{timestamp}\n请及时清理磁盘空间。"
    )
    for attempt in range(3):
        try:
            response = api_request(args.gateway_url, "/api/v1/touchpoints/wecom/send",
                                   {"to_user": args.to_user, "text": message})
            if response.get("ok") is not True:
                raise ValueError("WeCom send did not confirm delivery")
            print("alert delivered to " + args.to_user, flush=True)
            return
        except Exception:
            if attempt == 2:
                raise
            time.sleep(5)


def launchd_config(args):
    runtime = ROOT / "data/runtime"
    return {
        "Label": LABEL,
        "ProgramArguments": ["/usr/bin/python3", str(Path(__file__).resolve()), "check",
                             "--path", args.path, "--threshold", str(args.threshold),
                             "--to-user", args.to_user, "--gateway-url", args.gateway_url],
        "WorkingDirectory": str(ROOT),
        "RunAtLoad": True,
        "StartInterval": args.interval_seconds,
        "StandardOutPath": str(runtime / "disk-monitor.log"),
        "StandardErrorPath": str(runtime / "disk-monitor.log"),
    }


def install(args):
    validate_recipient(args)
    disk_usage(args.path)
    PLIST.parent.mkdir(parents=True, exist_ok=True)
    (ROOT / "data/runtime").mkdir(parents=True, exist_ok=True)
    staging = PLIST.with_suffix(".plist.tmp")
    with staging.open("wb") as stream:
        plistlib.dump(launchd_config(args), stream)
    staging.chmod(0o644)
    staging.replace(PLIST)
    subprocess.run(["launchctl", "bootout", DOMAIN + "/" + LABEL], capture_output=True)
    subprocess.run(["launchctl", "enable", DOMAIN + "/" + LABEL], check=True)
    subprocess.run(["launchctl", "bootstrap", DOMAIN, str(PLIST)], check=True)
    print(f"installed {PLIST}; interval={args.interval_seconds}s; threshold>{args.threshold:g}%; target={args.to_user}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["check", "install", "print", "status", "uninstall"])
    parser.add_argument("--path", default="/System/Volumes/Data")
    parser.add_argument("--threshold", type=float, default=85)
    parser.add_argument("--to-user", default="frypan")
    parser.add_argument("--gateway-url", default="http://127.0.0.1:8080")
    parser.add_argument("--interval-seconds", type=int, default=14400)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    if not 0 < args.threshold < 100 or args.interval_seconds <= 0:
        parser.error("threshold must be between 0 and 100 and interval must be positive")
    if args.command == "check":
        check(args)
    elif args.command == "print":
        sys.stdout.buffer.write(plistlib.dumps(launchd_config(args)))
    elif sys.platform != "darwin":
        parser.error("launchd installation requires macOS")
    elif args.command == "install":
        if args.dry_run:
            parser.error("use check --dry-run or print to review without installing")
        install(args)
    elif args.command == "status":
        if PLIST.exists():
            print(str(PLIST), flush=True)
            subprocess.run(["launchctl", "print", DOMAIN + "/" + LABEL], check=True)
        else:
            print("disk monitor not installed")
    elif args.command == "uninstall":
        subprocess.run(["launchctl", "bootout", DOMAIN + "/" + LABEL], capture_output=True)
        PLIST.unlink(missing_ok=True)
        print("disk monitor removed")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("disk monitor failed: " + str(error), file=sys.stderr)
        sys.exit(1)
