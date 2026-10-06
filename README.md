# ElkonexportEB

Exports ELKON production records from Microsoft SQL Server to the application
server over an authenticated WebSocket connection. It polls for new records,
rechecks unfinished productions, and saves export progress in its JSON config.

## Quick start

1. Install Go 1.24.3 or newer and build the application:

	```sh
	go build -o elkonexport .
	```

2. Edit [config.json](config.json): replace the example endpoints, SQL credentials,
	database name, and production site details with your own values.
3. Run from the directory containing `query.sql`:

	```sh
	./elkonexport ./config.json
	```

For Windows, use `make prod` for a 64-bit executable or `make prod386` for
32-bit, then run:

```powershell
.\elkonexport.exe .\config.json
```

Without a config argument, the application looks for `<executable-name>.json`
in the working directory, for example `elkonexport.json`. Relative log paths and
`query.sql` are also resolved from the working directory. Restart the
application after editing settings.

## Configuration

The demo includes every active setting and the compatibility debounce setting.
Timing settings use integer seconds; `_h` and `_hours` use hours, and
`max_day_age` uses days.

| Key | Purpose |
| --- | --- |
| `websocket_url` | WebSocket endpoint (`ws://` or `wss://`), without a trailing slash or query. The application appends the access token. |
| `mssql_conn_str` | SQL Server connection string, including the database and credentials. |
| `auth.url` | HTTP(S) login endpoint without a query string. Authentication uses the client's MAC address; there are no username/password settings for this endpoint. |
| `query_interval_s` | Delay after each polling cycle; default `10` when omitted or nonpositive. |
| `connect_retry_s` | Retry delay for SQL, login, and WebSocket connections; default `30` when omitted or nonpositive. |
| `production_error_debounce_s` | Suppress repeated identical error messages sent to the server for this duration; the demo uses `30`. See the debounce note below. |
| `empty_debounce_s` | Compatibility setting; empty results currently send a ping on every poll. Keep this positive to use the configured error debounce duration. |
| `shutdown_s` | Graceful shutdown timeout; default `10` when omitted or nonpositive. |
| `log_level` | `debug`, `info`, `warn`, or `error`. Omitted or unrecognized values use `debug`; the demo uses `info`. |
| `log_file_name` | Active log path. An empty value sends logs to standard error. |
| `production.query_production_count` | Maximum records selected per production query; default `1` when omitted or nonpositive. |
| `production.production_site_id` | Site identifier used in outgoing messages; set it to the site's ID on the server. |
| `production.production_site_descr` | Informational site description. |
| `production.check_unclosed_productions_h` | Recheck tracked unfinished productions during each poll while their tracking age is within this many hours; default `1` when omitted or nonpositive. Older entries are skipped. |
| `production.last_production` | Saved export checkpoint: new production queries select IDs greater than this value. The demo starts at `0`. |
| `production.unclosed_productions` | Saved unfinished-production tracking: an object with production IDs as keys and RFC3339 tracking timestamps as values. Start with `{}` for a new installation. |
| `log_rotation.interval_hours` | UTC-aligned rotation period; default `24`, rotating at midnight UTC. |
| `log_rotation.max_backups` | Maximum archived files retained, excluding the active file; default `5`. |
| `log_rotation.max_day_age` | Delete archives older than this many 24-hour days; default `30`. Both retention limits apply. |
| `log_rotation.compress` | Gzip completed log files; default `true`. Set `false` for plain archives. |

In the current code, `empty_debounce_s <= 0` forces the error debounce to
30 seconds regardless of `production_error_debounce_s`. With a positive
`empty_debounce_s`, the configured error debounce is used directly; zero or
negative error debounce values provide no suppression. The demo sets both to
`30`.

For the three numeric rotation settings, omission or zero uses the defaults;
negative values are rejected. The legacy `LogRotation` section is accepted, and
the former `max_mb_size` option is ignored because rotation is time-based.
Cleanup runs on startup, after rotation, and during background maintenance, even
when idle. See [LOGGING.md](LOGGING.md) for archive naming and upgrade details.

The application updates `last_production` and `unclosed_productions`
automatically. The config file and its directory must be writable. When
upgrading an existing installation, preserve these values; replacing them with
the demo's initial state can replay records. Stop the application before editing
the live config so a progress update does not overwrite your changes.
