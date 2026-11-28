-- Run with psql as the panel schema owner. Runtime roles must already exist.
GRANT USAGE ON SCHEMA public TO panel_serve, panel_issuer;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  public.runtime_settings, public.usage_observations, public.usage_snapshots,
  public.usage_versions, public.usage_cursors, public.patrol_policies,
  public.probe_templates, public.patrol_runs,
  public.ledger_accounts, public.ledger_alerts, public.ledger_days,
  public.ledger_events, public.ledger_hours, public.ledger_samples, public.ledger_wallets
TO panel_serve;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  public.runtime_settings, public.grants, public.issues, public.grant_audits
TO panel_issuer;

-- Grant access only to sequences owned by the tables listed above.
WITH owned(table_name, runtime_role) AS (
  SELECT unnest(ARRAY[
    'runtime_settings', 'usage_observations', 'usage_snapshots', 'usage_versions',
    'usage_cursors', 'patrol_policies', 'probe_templates', 'patrol_runs',
    'ledger_accounts', 'ledger_alerts', 'ledger_days', 'ledger_events',
    'ledger_hours', 'ledger_samples', 'ledger_wallets'
  ]), 'panel_serve'
  UNION ALL
  SELECT unnest(ARRAY['runtime_settings', 'grants', 'issues', 'grant_audits']), 'panel_issuer'
)
SELECT format('GRANT USAGE, SELECT ON SEQUENCE %s TO %I;',
              pg_get_serial_sequence(format('public.%I', table_name), 'id'), runtime_role)
FROM owned
WHERE pg_get_serial_sequence(format('public.%I', table_name), 'id') IS NOT NULL
\gexec
