import { createClient, type SupabaseClient } from '@supabase/supabase-js';

import type { Env } from '../config/env.js';

export function createSupabaseAdminClient(env: Env): SupabaseClient {
  return createClient(env.SUPABASE_URL, env.SUPABASE_SERVICE_ROLE_KEY, {
    auth: {
      autoRefreshToken: false,
      persistSession: false,
      detectSessionInUrl: false
    }
  });
}
