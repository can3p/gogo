-- +migrate Up
CREATE TABLE authors (
    id bigserial PRIMARY KEY,
    name text NOT NULL
);

-- +migrate Down
DROP TABLE authors;
