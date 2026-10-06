# Log rotation

The log rotates by time, with one file per 24-hour UTC day by default. The
configured active filename stays unchanged, for example `elkonexport.log`.
At the next period boundary, the previous file becomes an archive with a UTC
period timestamp in its name and a new active file is opened. Restarting the
application does not start a new period.

For example, the UTC day beginning October 6, 2026 is archived as
`elkonexport-2026-10-06T00-00-00.000000000Z.log.gz` when compression is enabled.
Existing archives with the old lumberjack timestamp format are also included in
cleanup. Only archives matching the configured log filename are managed.

On the first startup after upgrading, the existing active file is assigned to
the period of its last modification. Historical entries already mixed in that
file cannot be separated into individual days. Subsequent files follow the new
time periods.

Add these settings to the application JSON configuration:

```json
{
	"log_level": "info",
	"log_file_name": "elkonexport.log",
	"log_rotation": {
		"interval_hours": 24,
		"max_backups": 5,
		"max_day_age": 30,
		"compress": true
	}
}
```

`interval_hours` controls the duration of each UTC-aligned period. With the
default value of `24`, the period starts at `00:00 UTC` and ends at the next
`00:00 UTC`, so each period is exactly 24 hours, including across daylight-saving
changes. For example, `12` creates two periods per UTC day.

Old archives are deleted automatically on startup, after rotation, and during
background maintenance, including when no new messages are written. Both
retention limits apply: at most `max_backups` archives are kept, and archives
older than `max_day_age` days are removed. The active file is not counted among
the backups. The defaults keep at most five backups, each no older than 30 days.

`compress: true` writes gzip archives; set it to `false` to retain plain log
files. Omitting the setting enables compression. Omitting or setting
`interval_hours`, `max_backups`, or `max_day_age` to zero uses the respective
default. Negative values and time values that exceed the supported duration
range are rejected when the configuration is loaded.

The old JSON property `LogRotation` remains supported. Saved configurations use
`log_rotation`. The former `max_mb_size` setting is accepted for compatibility
and ignored: file size no longer causes rotation, and there is no size cap for a
single period.

Restart the application or its Windows service after changing rotation
settings. Configuration reloads alone do not replace the running log writer.
