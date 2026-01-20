-- Add additional user fields to support full user profile
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS name VARCHAR(255),
ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active',
ADD COLUMN IF NOT EXISTS is_phone_verified BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS enrollment_number VARCHAR(100),
ADD COLUMN IF NOT EXISTS programme VARCHAR(100),
ADD COLUMN IF NOT EXISTS course VARCHAR(255),
ADD COLUMN IF NOT EXISTS year VARCHAR(10),
ADD COLUMN IF NOT EXISTS expiry_date DATE,
ADD COLUMN IF NOT EXISTS hostel VARCHAR(100),
ADD COLUMN IF NOT EXISTS profile_picture TEXT,
ADD COLUMN IF NOT EXISTS disability_type VARCHAR(100),
ADD COLUMN IF NOT EXISTS disability_percentage DECIMAL(5,2),
ADD COLUMN IF NOT EXISTS udid_number VARCHAR(100),
ADD COLUMN IF NOT EXISTS disability_certificate TEXT,
ADD COLUMN IF NOT EXISTS id_proof_type VARCHAR(50),
ADD COLUMN IF NOT EXISTS id_proof_document TEXT,
ADD COLUMN IF NOT EXISTS license_number VARCHAR(100),
ADD COLUMN IF NOT EXISTS vehicle_number VARCHAR(100),
ADD COLUMN IF NOT EXISTS vehicle_type VARCHAR(50);

-- Update role column to support more roles
ALTER TABLE users 
ALTER COLUMN role TYPE VARCHAR(50);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_enrollment_number ON users(enrollment_number);

-- Update existing users to have active status
UPDATE users SET status = 'active' WHERE status IS NULL;
