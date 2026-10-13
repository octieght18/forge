// Package retention states how long product data is kept and where secrets live.
// Demo cleanup removes only named synthetic files. It does not open PostgreSQL.
package retention

// Class is one retention rule. It stores no secret value.
type Class struct {
	Name      string
	Store     string
	Retention string
	Deletion  string
}

// SecretPlace names a credential location without its value.
type SecretPlace struct {
	Name  string
	Place string
}

// Classes returns the retention rules for executions, evidence, audit, and attachments.
func Classes() []Class {
	return []Class{
		{
			Name:      "executions",
			Store:     "PostgreSQL runs and start/cancel commands; Temporal history for workflow forge-run/<run UUID>",
			Retention: "Product rows stay for the life of the product database. Temporal history stays with that workflow. No expiry clock is set.",
			Deletion:  "Principals, versions, and runs reject DELETE. Demo cleanup does not connect to PostgreSQL or Temporal.",
		},
		{
			Name:      "evidence",
			Store:     "Worker process memory",
			Retention: "A passage is kept as workflow_history until compensation marks it compensated. Process exit drops the list.",
			Deletion:  "There is no product evidence table. Demo cleanup does not clear worker memory.",
		},
		{
			Name:      "audit",
			Store:     "API and worker process",
			Retention: "Events stay until the process exits. An event has actor, tenant, operation, time, target, and outcome.",
			Deletion:  "Audit events are not written to PostgreSQL. Prompts and tokens are not fields. Demo cleanup does not erase the process record.",
		},
		{
			Name:      "attachments",
			Store:     "none",
			Retention: "The API does not accept attachments.",
			Deletion:  "Nothing is stored, so nothing is deleted.",
		},
	}
}

// SecretPlaces lists credential homes. Values stay in those homes and out of Git.
func SecretPlaces() []SecretPlace {
	return []SecretPlace{
		{Name: "FORGE_DATABASE_URL", Place: "environment"},
		{Name: "FORGE_MIGRATION_DATABASE_URL", Place: "environment"},
		{Name: "FORGE_MODEL_API_KEY", Place: "environment"},
		{Name: "postgres-admin", Place: "kubernetes-secret"},
		{Name: "keycloak-db", Place: "kubernetes-secret"},
		{Name: "api-db", Place: "kubernetes-secret"},
		{Name: "migration-db", Place: "kubernetes-secret"},
		{Name: "cursor-key", Place: "kubernetes-secret"},
		{Name: "secrets.json", Place: "local-file"},
		{Name: "cursor.key", Place: "local-file"},
		{Name: "kubeconfig", Place: "local-file"},
	}
}
