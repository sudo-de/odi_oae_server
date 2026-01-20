-- Backfill location data for existing sessions
-- Sets location to IP address for sessions where location is NULL
-- This ensures existing sessions show IP address instead of "Location unavailable"

UPDATE sessions
SET location = ip_address
WHERE location IS NULL 
  AND ip_address IS NOT NULL 
  AND ip_address != '';

-- Note: For more accurate location data, you would need to run a Go script
-- that calls the geolocation API for each IP address. This migration provides
-- a basic fallback so existing sessions at least show their IP address.
