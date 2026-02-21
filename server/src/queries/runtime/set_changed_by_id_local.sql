-- Set audit actor ID as transaction local setting
SELECT set_config('app.changed_by_id', $1, false);
