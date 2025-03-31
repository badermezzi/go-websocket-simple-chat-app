-- ALTER TABLE "messages" DROP CONSTRAINT IF EXISTS messages_sender_id_fkey;
-- ALTER TABLE "messages" DROP CONSTRAINT IF EXISTS messages_receiver_id_fkey;

DROP TABLE IF EXISTS "messages";
DROP TABLE IF EXISTS "users";