package configs_test

import (
	"testing"

	"github.com/Foundato/kelon/configs"
	"github.com/google/go-cmp/cmp"
)

//nolint:gochecknoglobals
var wantDatatoreConfig = configs.DatastoreConfig{
	Datastores: map[string]*configs.Datastore{
		"mysql": {
			Type: "mysql",
			Connection: map[string]string{
				"host":     "localhost",
				"port":     "5432",
				"database": "mysql",
				"user":     "mysql",
				"password": "SuperSecure",
			},
			Metadata: map[string]string{
				"default_schema": "default",
			},
		},
		"local-json": {
			Type: "file",
			Connection: map[string]string{
				"location": "./data/local-data.json",
			},
			Metadata: map[string]string{
				"in_memory": "true",
			},
		},
	},
	DatastoreSchemas: map[string]map[string]*configs.EntitySchema{
		"mysql": {
			"appstore": {
				Entities: []string{"users", "followers"},
			},
		},
	},
}

//nolint:gochecknoglobals
var wantAPIConfig = &configs.APIConfig{
	Mappings: []*configs.DatastoreAPIMapping{
		{
			Prefix:    "/api",
			Datastore: "mysql",
			Mappings: []*configs.APIMapping{
				{
					Path:    "/.*",
					Package: "default",
					Methods: nil,
					Queries: nil,
				},
				{
					Path:    "/articles",
					Package: "articles",
					Methods: []string{"POST"},
					Queries: nil,
				},
				{
					Path:    "/articles",
					Package: "articles",
					Methods: []string{"GET"},
					Queries: []string{"author"},
				},
			},
		},
	},
}

func TestLoadConfigFromFile(t *testing.T) {
	result, err := configs.FileConfigLoader{
		DatastoreConfigPath: "./testdata/datastore.yml",
		APIConfigPath:       "./testdata/api.yml",
	}.Load()

	if err != nil {
		t.Errorf("Unexpected error while parsing config: %s", err)
	}

	// Validate datastore config
	if have := result.Data; have != nil {
		if !cmp.Equal(wantDatatoreConfig, *have) {
			t.Errorf("Datastores config is not as expected! Diff: %s", cmp.Diff(wantDatatoreConfig, *have))
		}
	} else {
		t.Error("No datastore configuration present!")
	}

	// Validate api config
	if have := result.API; have != nil {
		if !cmp.Equal(wantAPIConfig, have) {
			t.Errorf("API config is not as expected! Diff: %s", cmp.Diff(wantAPIConfig, have))
		}
	} else {
		t.Error("No api configuration present!")
	}
}

func TestLoadNotExistingDatastoreFile(t *testing.T) {
	_, err := configs.FileConfigLoader{
		DatastoreConfigPath: "./datastore-not-existing.yml",
		APIConfigPath:       "./api-not-existing.yml",
	}.Load()

	if err == nil || err.Error() != "open ./datastore-not-existing.yml: no such file or directory" {
		t.Error("File not found error not thrown!")
	}
}

func TestLoadNotExistingApiFile(t *testing.T) {
	_, err := configs.FileConfigLoader{
		DatastoreConfigPath: "./testdata/datastore.yml",
		APIConfigPath:       "./api-not-existing.yml",
	}.Load()

	if err == nil || err.Error() != "open ./api-not-existing.yml: no such file or directory" {
		t.Error("File not found error not thrown!")
	}
}
