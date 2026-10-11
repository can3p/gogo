-- +migrate Up
CREATE TABLE app_things (id int);

-- +migrate Down
DROP TABLE app_things;
