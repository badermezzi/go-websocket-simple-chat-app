CREATE TABLE "users" (
  "id" SERIAL PRIMARY KEY,
  "username" varchar(50) UNIQUE NOT NULL,
  "password_plaintext" text NOT NULL,
  "status" varchar(10) NOT NULL DEFAULT 'offline',
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "messages" (
  "id" bigserial PRIMARY KEY,
  "sender_id" int NOT NULL,
  "receiver_id" int NOT NULL,
  "content" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

COMMENT ON COLUMN "users"."password_plaintext" IS 'Practice only!';

ALTER TABLE "messages" ADD FOREIGN KEY ("sender_id") REFERENCES "users" ("id");

ALTER TABLE "messages" ADD FOREIGN KEY ("receiver_id") REFERENCES "users" ("id");
