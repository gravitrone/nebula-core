-- Set audit actor ID in session state
SELECT set_config('app.changed_by_id', $1, true);
