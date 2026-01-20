-- Add enrollment expiry and max students functionality
-- Migration: 006_add_enrollment_expiry.sql

-- Add max_students field to courses table
ALTER TABLE courses ADD COLUMN IF NOT EXISTS max_students INTEGER DEFAULT 5;

-- Add expiry_date field to course_students table
ALTER TABLE course_students ADD COLUMN IF NOT EXISTS expiry_date TIMESTAMP WITH TIME ZONE;

-- Create index for faster expiry date queries
CREATE INDEX IF NOT EXISTS idx_course_students_expiry_date ON course_students(expiry_date);

-- Update existing enrollments to have no expiry (they remain active indefinitely)
-- This preserves existing data while allowing new enrollments to have expiry dates