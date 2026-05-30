DROP TRIGGER IF EXISTS trg_session_memories_updated_at ON session_memories;
DROP FUNCTION IF EXISTS update_session_memories_updated_at();
DROP INDEX IF EXISTS idx_session_memories_user_updated;
DROP TABLE IF EXISTS session_memories;
