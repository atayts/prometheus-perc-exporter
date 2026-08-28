Prometheus LSI RAID controller exporter
=======================================

Monitors the status of LSI controllers and creates alerts if anything goes wrong.

Configuration
-------------

All options live in `config.yml`, which the exporter reads from its own
directory (`C:\Program Files\prometheus-perc-exporter\config.yml` for the
Chocolatey package). To change an option, edit the file and restart the
service — no reinstall is needed:

```powershell
notepad 'C:\Program Files\prometheus-perc-exporter\config.yml'
Restart-Service perc_win_exporter
```

Every key is optional; an omitted key keeps its built-in default.

| Key | Default | Meaning |
| --- | --- | --- |
| `perccli_path` | `C:\ProgramData\chocolatey\bin\perccli64.exe` | Path to the perccli64 executable. |
| `web_listen_address` | `:9102` | Address to listen on for metrics. |
| `scrape_interval` | `900` | How often to scrape PERC data, in seconds. |
| `log_file` | *(empty)* | Log destination. Empty means `perc_win_exporter.log` next to the executable when running as a service, or stderr in console mode. Rotated to `<name>.old` past 10 MB. |

The service fails to start on an invalid configuration and writes the reason to
the log file.

Command line
------------

The service takes no arguments — it is configured entirely by `config.yml`. The
flags below exist for ad-hoc console runs and override the file.

```
usage: perc_win_exporter.exe [<flags>]

Flags:
  -h, --[no-]help              Show context-sensitive help (also try --help-long
                               and --help-man).
      --config.file=""         Path to the configuration file (default:
                               config.yml next to the executable).
      --perccli.path=""        Override perccli_path from the configuration
                               file.
      --web.listen-address=""  Override web_listen_address from the
                               configuration file.
      --scrape.interval=0      Override scrape_interval from the configuration
                               file.
      --[no-]version           Show application version.
```

[Download](https://github.com/atayts/prometheus-perc-exporter/releases/latest/download/perc_win_exporter.exe) the latest release.
