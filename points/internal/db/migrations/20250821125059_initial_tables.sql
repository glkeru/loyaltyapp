-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS accounts (
  uuid         uuid PRIMARY KEY,
  userid       text NOT NULL,
  balance      numeric(18,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tnx (
  id           uuid PRIMARY KEY,
  pointaccount uuid NOT NULL,
  points       numeric(18,2) NOT NULL,
  commitdate  timestamptz   NOT NULL,
  commit       boolean       NOT NULL DEFAULT false,
  typetnx      int           NOT NULL,
  orderid      text,
  transferid   text,
  redeemid     text,
  CONSTRAINT points_nonzero CHECK (points <> 0)
);

CREATE INDEX IF NOT EXISTS idx_accounts_user
  ON accounts (userid);

CREATE INDEX IF NOT EXISTS idx_tnx_commitdate
  ON tnx (commitdate);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_accounts_user;
DROP INDEX IF EXISTS idx_tnx_commitdate;
DROP TABLE IF EXISTS tnx;
DROP TABLE IF EXISTS accounts;
-- +goose StatementEnd
