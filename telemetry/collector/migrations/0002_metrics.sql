-- Aggregate dimensions only. Preserve existing schema-1 start/heartbeat counts.
CREATE TABLE metric_counts (
    minute INTEGER NOT NULL,
    version TEXT NOT NULL,
    metric TEXT NOT NULL,
    value TEXT NOT NULL,
    count INTEGER NOT NULL CHECK (count >= 0),
    PRIMARY KEY (minute, version, metric, value)
);
INSERT INTO metric_counts SELECT minute, version, 'start', 'total', starts FROM counts WHERE starts > 0;
INSERT INTO metric_counts SELECT minute, version, 'heartbeat', 'total', heartbeats FROM counts WHERE heartbeats > 0;
DROP TABLE counts;
ALTER TABLE metric_counts RENAME TO counts;
