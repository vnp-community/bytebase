package common

import (
	"testing"
)

func TestParseProjectRef(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantErr   bool
	}{
		{"projects/p1", "p1", false},
		{"projects/my-project", "my-project", false},
		{"invalid", "", true},
		{"projects/", "", false}, // empty project ID
		{"instances/i1", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseProjectRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProjectRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil && ref.ProjectID != tt.wantID {
				t.Errorf("ParseProjectRef(%q).ProjectID = %q, want %q", tt.input, ref.ProjectID, tt.wantID)
			}
		})
	}
}

func TestParseResourcePlanRef(t *testing.T) {
	tests := []struct {
		input       string
		wantProject string
		wantUID     int64
		wantErr     bool
	}{
		{"projects/p1/plans/123", "p1", 123, false},
		{"projects/p1/plans/abc", "", 0, true},
		{"invalid", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseResourcePlanRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResourcePlanRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil {
				if ref.ProjectID != tt.wantProject {
					t.Errorf("ProjectID = %q, want %q", ref.ProjectID, tt.wantProject)
				}
				if ref.PlanUID != tt.wantUID {
					t.Errorf("PlanUID = %d, want %d", ref.PlanUID, tt.wantUID)
				}
			}
		})
	}
}

func TestParseDatabaseResourceRef(t *testing.T) {
	tests := []struct {
		input        string
		wantInstance string
		wantDB       string
		wantErr      bool
	}{
		{"instances/i1/databases/db1", "i1", "db1", false},
		{"instances/prod-instance/databases/mydb", "prod-instance", "mydb", false},
		{"invalid", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseDatabaseResourceRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if ref.InstanceID != tt.wantInstance {
					t.Errorf("InstanceID = %q, want %q", ref.InstanceID, tt.wantInstance)
				}
				if ref.DatabaseName != tt.wantDB {
					t.Errorf("DatabaseName = %q, want %q", ref.DatabaseName, tt.wantDB)
				}
			}
		})
	}
}

func TestResourceRefRoundTrip(t *testing.T) {
	// Verify that Parse(ref.String()) == ref for all types
	plan := ResourcePlanRef{ProjectID: "proj-1", PlanUID: 42}
	parsed, err := ParseResourcePlanRef(plan.String())
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if parsed.ProjectID != plan.ProjectID || parsed.PlanUID != plan.PlanUID {
		t.Errorf("round-trip mismatch: got %+v, want %+v", parsed, plan)
	}

	db := DatabaseResourceRef{InstanceID: "inst-1", DatabaseName: "mydb"}
	parsedDB, err := ParseDatabaseResourceRef(db.String())
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if parsedDB.InstanceID != db.InstanceID || parsedDB.DatabaseName != db.DatabaseName {
		t.Errorf("round-trip mismatch: got %+v, want %+v", parsedDB, db)
	}
}
