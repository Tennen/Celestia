import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("disk_monitor", Path(__file__).with_name("disk-monitor.py"))
monitor = importlib.util.module_from_spec(spec)
spec.loader.exec_module(monitor)


class DiskMonitorTests(unittest.TestCase):
    def test_threshold_is_strictly_greater(self):
        self.assertFalse(monitor.exceeds_threshold({"used_percent": 85}, 85))
        self.assertTrue(monitor.exceeds_threshold({"used_percent": 85.001}, 85))

    def test_df_mount_with_spaces_and_available_capacity(self):
        usage = monitor.parse_usage("Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/disk3s5 1200 850 150 85% /Volumes/My Data\n")
        self.assertEqual(usage["mount"], "/Volumes/My Data")
        self.assertEqual(usage["used_percent"], 85)

    def test_invalid_df_does_not_become_a_healthy_check(self):
        for output in ["", "header\n/dev/disk 0 0 0 0% /\n", "header\nbroken\n"]:
            with self.subTest(output=output), self.assertRaises(ValueError):
                monitor.parse_usage(output)

    def test_normal_usage_and_dry_run_never_send(self):
        from argparse import Namespace
        for percent, dry_run in [(85, False), (90, True)]:
            args = Namespace(path="/", threshold=85, dry_run=dry_run)
            with patch.object(monitor, "disk_usage", return_value={"used_percent": percent}), patch.object(monitor, "api_request") as send:
                monitor.check(args)
                send.assert_not_called()

    def test_schedule_survives_login_with_exact_interval_and_target(self):
        from argparse import Namespace
        config = monitor.launchd_config(Namespace(path="/System/Volumes/Data", threshold=85, to_user="frypan", gateway_url="http://127.0.0.1:8080", interval_seconds=14400))
        self.assertTrue(config["RunAtLoad"])
        self.assertEqual(config["StartInterval"], 14400)
        self.assertIn("frypan", config["ProgramArguments"])

    def test_alert_uses_configured_recipient_and_reports_real_measurement(self):
        from argparse import Namespace
        args = Namespace(path="/", threshold=85, dry_run=False, to_user="frypan", gateway_url="http://127.0.0.1:8080")
        usage = {"used_percent": 86, "mount": "/System/Volumes/Data", "used_gib": 172, "available_gib": 28}
        with patch.object(monitor, "disk_usage", return_value=usage), patch.object(monitor, "api_request", return_value={"ok": True}) as send:
            monitor.check(args)
            send.assert_called_once()
            url, route, payload = send.call_args.args
            self.assertEqual(route, "/api/v1/touchpoints/wecom/send")
            self.assertEqual(payload["to_user"], "frypan")
            self.assertIn("86.00%", payload["text"])
            self.assertIn("28.0 GiB", payload["text"])

    def test_failed_delivery_retries_and_remains_a_failure(self):
        from argparse import Namespace
        args = Namespace(path="/", threshold=85, dry_run=False, to_user="frypan", gateway_url="http://127.0.0.1:8080")
        usage = {"used_percent": 86, "mount": "/", "used_gib": 86, "available_gib": 14}
        with patch.object(monitor, "disk_usage", return_value=usage), patch.object(monitor, "api_request", side_effect=OSError("gateway unavailable")) as send, patch.object(monitor.time, "sleep"):
            with self.assertRaises(OSError):
                monitor.check(args)
            self.assertEqual(send.call_count, 3)


if __name__ == "__main__":
    unittest.main()
