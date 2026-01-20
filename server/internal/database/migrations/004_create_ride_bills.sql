-- Create ride_bills table
CREATE TABLE IF NOT EXISTS ride_bills (
    id SERIAL PRIMARY KEY,
    ride_id INTEGER NOT NULL REFERENCES ride_locations(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_location VARCHAR(255) NOT NULL,
    to_location VARCHAR(255) NOT NULL,
    fare DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'cancelled')),
    driver VARCHAR(255),
    distance DECIMAL(10, 2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster searches
CREATE INDEX IF NOT EXISTS idx_ride_bills_ride_id ON ride_bills(ride_id);
CREATE INDEX IF NOT EXISTS idx_ride_bills_user_id ON ride_bills(user_id);
CREATE INDEX IF NOT EXISTS idx_ride_bills_status ON ride_bills(status);
CREATE INDEX IF NOT EXISTS idx_ride_bills_created_at ON ride_bills(created_at);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_ride_bills_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop trigger if exists, then create trigger to automatically update updated_at
DROP TRIGGER IF EXISTS trigger_update_ride_bills_updated_at ON ride_bills;
CREATE TRIGGER trigger_update_ride_bills_updated_at
    BEFORE UPDATE ON ride_bills
    FOR EACH ROW
    EXECUTE FUNCTION update_ride_bills_updated_at();
