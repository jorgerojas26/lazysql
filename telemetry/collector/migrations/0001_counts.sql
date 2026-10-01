-- Receipt times are rounded to a minute before storage. No event/identity table.
CREATE TABLE counts (
    minute INTEGER NOT NULL,
    version TEXT NOT NULL,
    starts INTEGER NOT NULL DEFAULT 0 CHECK (starts >= 0),
    heartbeats INTEGER NOT NULL DEFAULT 0 CHECK (heartbeats >= 0),
    PRIMARY KEY (minute, version)
);
