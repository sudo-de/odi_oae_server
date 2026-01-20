-- Add logged_out_at column to track when sessions were logged out
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS logged_out_at TIMESTAMP WITH TIME ZONE;

-- Create index for faster queries on logged out sessions
CREATE INDEX IF NOT EXISTS idx_sessions_logged_out_at ON sessions(logged_out_at);

-- Update expired sessions to set logged_out_at to their expires_at time
UPDATE sessions
SET logged_out_at = expires_at
WHERE expires_at < CURRENT_TIMESTAMP 
  AND logged_out_at IS NULL;
