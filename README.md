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

Building
--------

Requires Go 1.25 or newer. The exporter is Windows-only — it links the Windows
service API — so it must be built for `windows/amd64`:

```powershell
go build -o perc_win_exporter.exe .
```

From Linux or macOS, cross-compile:

```sh
GOOS=windows GOARCH=amd64 go build -o perc_win_exporter.exe .
```

Packaging
---------

The Chocolatey package expects the binary in `choco\tools\`. It is gitignored,
so build it there before packing. `config.yml` is pulled in from the repository
root by the `<files>` manifest in the nuspec, so there is only ever one copy of
it to keep up to date.

```powershell
go build -o choco\tools\perc_win_exporter.exe .
choco pack choco\prometheus-perc-exporter.nuspec
```

That writes `prometheus-perc-exporter.<version>.nupkg` to the current directory.
Bump `<version>` in the nuspec and the `version` constant in `exporter.go`
together — the exporter reports the latter as the `version` label on
`perc_exporter_info`.

To install the package from the local directory as a smoke test:

```powershell
choco install prometheus-perc-exporter -s . -y
Get-Service perc_win_exporter
Invoke-WebRequest http://localhost:9102/metrics -UseBasicParsing
```

Upgrading in place keeps an existing
`C:\Program Files\prometheus-perc-exporter\config.yml`; the packaged defaults
land next to it as `config.yml.example`.

[Download](https://github.com/atayts/prometheus-perc-exporter/releases/latest/download/perc_win_exporter.exe) the latest release.
