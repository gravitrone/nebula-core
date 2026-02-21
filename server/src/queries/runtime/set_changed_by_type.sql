-- Set audit actor type in session state
SELECT set_config('app.changed_by_type', $1, true);
