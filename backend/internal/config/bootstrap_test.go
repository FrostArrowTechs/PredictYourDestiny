package config

import "testing"

func TestNormalizeDBNameSupportsURLAndKeywordDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{"URL postgres", "postgres://user:p%40ss@db.example.com:5432/postgres?sslmode=require", "predictdestiny"},
		{"URL custom", "postgresql://user:pass@db.example.com/custom-db?sslmode=require", "custom-db"},
		{"keyword quoted password", "host=db.example.com user=test password='two words' dbname=postgres sslmode=require", "predictdestiny"},
		{"keyword custom", "host=db.example.com user=test password='two words' dbname=my_app sslmode=require", "my_app"},
		{"keyword omitted database", "host=db.example.com user=test password='two words' sslmode=require", "predictdestiny"},
		{"URL omitted database", "postgres://user:pass@db.example.com/?sslmode=require", "predictdestiny"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := normalizeDBName(tt.dsn, "predictdestiny")
			if err != nil {
				t.Fatal(err)
			}
			if config.Database != tt.want {
				t.Fatalf("database = %q, want %q", config.Database, tt.want)
			}
			if config.Password != "p@ss" && config.Password != "pass" && config.Password != "two words" {
				t.Fatalf("password was not preserved: %q", config.Password)
			}
		})
	}
}

func TestSplitDSNPreservesConnectionAndUsesAdminDatabase(t *testing.T) {
	dsn := "postgres://user:p%40ss@db.example.com:5444/app-prod?sslmode=require&application_name=pyd"
	target, admin, err := splitDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if target != "app-prod" || admin.Database != "postgres" {
		t.Fatalf("target=%q admin=%q", target, admin.Database)
	}
	if admin.Host != "db.example.com" || admin.Port != 5444 || admin.User != "user" || admin.Password != "p@ss" {
		t.Fatalf("admin connection fields were not preserved: %+v", admin)
	}
	if admin.RuntimeParams["application_name"] != "pyd" {
		t.Fatalf("runtime params were not preserved: %+v", admin.RuntimeParams)
	}
}

func TestNormalizeDBNameRejectsInvalidDSN(t *testing.T) {
	if _, err := normalizeDBName("postgres://%zz", "predictdestiny"); err == nil {
		t.Fatal("invalid DSN was accepted")
	}
}

func TestSplitCSVTrimsAndDropsEmptyOrigins(t *testing.T) {
	got := splitCSV(" https://app.example.com/, ,https://admin.example.com ")
	if len(got) != 2 || got[0] != "https://app.example.com" || got[1] != "https://admin.example.com" {
		t.Fatalf("splitCSV() = %#v", got)
	}
}
