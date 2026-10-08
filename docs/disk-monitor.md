# Local disk monitoring

The macOS operations job checks the filesystem containing `/System/Volumes/Data`
every four hours and on login. Only usage strictly above 85% sends an alert to
the existing enabled WeCom recipient `frypan`, through Celestia's
`POST /api/v1/touchpoints/wecom/send` API. Normal checks are silent in WeCom.
Each subsequent check above the threshold sends another alert.

```bash
/usr/bin/python3 tool/disk-monitor.py install
/usr/bin/python3 tool/disk-monitor.py status
/usr/bin/python3 tool/disk-monitor.py check --dry-run
```

The job is saved as `~/Library/LaunchAgents/com.celestia.disk-monitor.plist`
with `StartInterval=14400` and `RunAtLoad=true`. It resumes after reboot and
user login. macOS does not wake a sleeping computer for this job; missed sleep
intervals are coalesced into one check after wake. It runs outside the gateway
process, and sends through the gateway's existing Touchpoints implementation.
No LLM or terminal capability needs to be enabled.

Disk statistics come from `/bin/df -kP`. Usage is calculated as
`used / (used + available) * 100`, matching the available-capacity basis of
`df` without rounding before comparing with 85%. This matters for macOS APFS
volumes that share container capacity. Alerts include hostname, mount,
percentage, used and available GiB, and the check time.

Installer options include `--path`, `--threshold`, `--to-user`,
`--gateway-url`, and `--interval-seconds`. Reinstall after moving the repository
or changing configuration. The installer verifies real filesystem statistics
and that the recipient is configured and enabled before installing.

Check history and delivery errors are in `data/runtime/disk-monitor.log`.
Failed sends retry twice after five-second waits, then fail visibly in that log
and the launchd exit status. The gateway must be running to send; the next
scheduled check tries again if disk usage is still high.

To review or remove the configuration:

```bash
/usr/bin/python3 tool/disk-monitor.py print
/usr/bin/python3 tool/disk-monitor.py uninstall
```
