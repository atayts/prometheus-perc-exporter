Prometheus LSI RAID controller exporter
=======================================

Monitors the status of LSI controllers and creates alerts if anything goes wrong.

```
usage: perc_win_exporter.exe [<flags>]


Flags:
  -h, --[no-]help            Show context-sensitive help (also try --help-long
                             and --help-man).
      --perccli.path="C:\\ProgramData\\chocolatey\\bin\\perccli64.exe"
                             Path to perccli64 executable.
      --web.listen-address=":9102"
                             Address to listen on for metrics.
      --scrape.interval=900  How often to scrape PERC data in seconds.
      --[no-]version         Show application version.
```

[Download](https://github.com/atayts/prometheus-perc-exporter/releases/latest/download/perc_win_exporter.exe) the latest release.
