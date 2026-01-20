-- Create ride_locations table
CREATE TABLE IF NOT EXISTS ride_locations (
    id SERIAL PRIMARY KEY,
    from_location VARCHAR(255) NOT NULL,
    to_location VARCHAR(255) NOT NULL,
    fare DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(from_location, to_location)
);

-- Create index for faster searches
CREATE INDEX IF NOT EXISTS idx_ride_locations_from ON ride_locations(from_location);
CREATE INDEX IF NOT EXISTS idx_ride_locations_to ON ride_locations(to_location);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_ride_locations_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop trigger if exists, then create trigger to automatically update updated_at
DROP TRIGGER IF EXISTS trigger_update_ride_locations_updated_at ON ride_locations;
CREATE TRIGGER trigger_update_ride_locations_updated_at
    BEFORE UPDATE ON ride_locations
    FOR EACH ROW
    EXECUTE FUNCTION update_ride_locations_updated_at();
