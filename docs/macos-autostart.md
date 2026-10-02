# macOS autostart

Install Celestia's login startup job once, after building the gateway:

```bash
./tool/celestia-autostart.sh install
./tool/celestia-autostart.sh status
```

The installer saves `~/Library/LaunchAgents/com.celestia.autostart.plist`, enables
and loads it, and checks that the gateway starts. macOS runs this job whenever
the current user logs in, including after a reboot. It requires a user login;
it does not run before login or while the computer is asleep.

The startup job calls `tool/celestia-service.sh start` with the repository's
absolute path and working directory. That script manages the gateway and its
plugins as usual. Repeated starts detect an already running gateway. The login
job runs once per login, so a manual service stop remains effective for that
session. Continue to use `./tool/redeploy.sh` for rebuilds and restarts.

The installer captures `CELESTIA_ADDR`, `CELESTIA_RUNTIME_DIR`,
`CELESTIA_GATEWAY_BIN`, `CELESTIA_GATEWAY_PID_FILE`, `CELESTIA_GATEWAY_LOG_FILE`,
`CELESTIA_LAUNCHD_LABEL`, and `CELESTIA_SERVICE_PATH` if supplied. The default
service PATH includes Homebrew and the installer's current PATH, preserving
versioned tool locations such as nvm's Node directory. Startup does not depend
on an interactive shell profile. Reinstall after moving the checkout, changing
these values, or removing a captured tool version. The gateway binary can be
rebuilt in place.

Startup output is written to `data/runtime/autostart.log`; gateway output remains
in `data/runtime/gateway.log` (under `CELESTIA_RUNTIME_DIR` if overridden).

To remove automatic startup:

```bash
./tool/celestia-autostart.sh uninstall
```

This removes only the login job; stop the running gateway separately with
`./tool/celestia-service.sh stop` if needed. To review the generated launchd
configuration without installing it, run `./tool/celestia-autostart.sh print`.
