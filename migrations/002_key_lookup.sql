CREATE OR REPLACE FUNCTION lookup_api_key(p_hash text)
RETURNS TABLE (id uuid, customer_id uuid, revoked_at timestamptz)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
  SELECT id, customer_id, revoked_at
  FROM api_keys
  WHERE key_hash = p_hash;
$$;

REVOKE ALL ON FUNCTION lookup_api_key(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION lookup_api_key(text) TO app;
