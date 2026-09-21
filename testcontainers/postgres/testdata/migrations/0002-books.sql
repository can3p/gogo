-- +migrate Up
CREATE TABLE books (
    id bigserial PRIMARY KEY,
    author_id bigint NOT NULL REFERENCES authors (id),
    title text NOT NULL
);

-- +migrate Down
DROP TABLE books;
