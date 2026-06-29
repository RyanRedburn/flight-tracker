package store

const (
	QueryCreateJob = `
		INSERT INTO jobs (id, type, payload, status, result, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	QueryGetJob = `
		SELECT id, type, payload, status, result, error, created_at, updated_at
		FROM jobs
		WHERE id = ?`

	QueryListJobs = `
		SELECT id, type, payload, status, result, error, created_at, updated_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT ?`

	QueryUpdateJob = `
		UPDATE jobs
		SET type = ?, payload = ?, status = ?, result = ?, error = ?, updated_at = ?
		WHERE id = ?`

	QueryMigrationVersion = `
		SELECT version, dirty
		FROM schema_migrations
		LIMIT 1`
)
