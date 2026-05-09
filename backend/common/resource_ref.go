package common

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ============================================================
// Typed Resource Reference Structs (TASK-AI-004-1)
//
// These structs provide type-safe, self-documenting resource name
// handling. Each struct corresponds to an AIP resource pattern.
// ============================================================

// ProjectRef represents "projects/{project}".
type ProjectRef struct{ ProjectID string }

func (r ProjectRef) String() string {
	return fmt.Sprintf("projects/%s", r.ProjectID)
}

// PlanRef represents "projects/{project}/plans/{planUID}".
type ResourcePlanRef struct {
	ProjectID string
	PlanUID   int64
}

func (r ResourcePlanRef) String() string {
	return fmt.Sprintf("projects/%s/plans/%d", r.ProjectID, r.PlanUID)
}

// IssueResourceRef represents "projects/{project}/issues/{issueUID}".
type IssueResourceRef struct {
	ProjectID string
	IssueUID  int64
}

func (r IssueResourceRef) String() string {
	return fmt.Sprintf("projects/%s/issues/%d", r.ProjectID, r.IssueUID)
}

// ReleaseRef represents "projects/{project}/releases/{releaseID}".
type ReleaseRef struct {
	ProjectID string
	ReleaseID string
}

func (r ReleaseRef) String() string {
	return fmt.Sprintf("projects/%s/releases/%s", r.ProjectID, r.ReleaseID)
}

// RolloutRef represents "projects/{project}/plans/{planUID}/rollout".
type RolloutRef struct {
	ProjectID string
	PlanUID   int64
}

func (r RolloutRef) String() string {
	return fmt.Sprintf("projects/%s/plans/%d/rollout", r.ProjectID, r.PlanUID)
}

// StageRef represents "projects/{project}/plans/{planUID}/rollout/stages/{stageID}".
type StageRef struct {
	ProjectID string
	PlanUID   int64
	StageID   string
}

func (r StageRef) String() string {
	return fmt.Sprintf("projects/%s/plans/%d/rollout/stages/%s", r.ProjectID, r.PlanUID, r.StageID)
}

// TaskResourceRef represents "projects/{project}/plans/{planUID}/rollout/stages/{stageID}/tasks/{taskUID}".
type TaskResourceRef struct {
	ProjectID string
	PlanUID   int64
	StageID   string
	TaskUID   int64
}

func (r TaskResourceRef) String() string {
	return fmt.Sprintf("projects/%s/plans/%d/rollout/stages/%s/tasks/%d", r.ProjectID, r.PlanUID, r.StageID, r.TaskUID)
}

// TaskRunResourceRef represents "projects/{project}/plans/{planUID}/rollout/stages/{stageID}/tasks/{taskUID}/taskRuns/{taskRunUID}".
type TaskRunResourceRef struct {
	ProjectID  string
	PlanUID    int64
	StageID    string
	TaskUID    int64
	TaskRunUID int64
}

func (r TaskRunResourceRef) String() string {
	return fmt.Sprintf("projects/%s/plans/%d/rollout/stages/%s/tasks/%d/taskRuns/%d",
		r.ProjectID, r.PlanUID, r.StageID, r.TaskUID, r.TaskRunUID)
}

// InstanceRef represents "instances/{instanceID}".
type InstanceRef struct{ InstanceID string }

func (r InstanceRef) String() string {
	return fmt.Sprintf("instances/%s", r.InstanceID)
}

// DatabaseResourceRef represents "instances/{instanceID}/databases/{databaseName}".
type DatabaseResourceRef struct {
	InstanceID   string
	DatabaseName string
}

func (r DatabaseResourceRef) String() string {
	return fmt.Sprintf("instances/%s/databases/%s", r.InstanceID, r.DatabaseName)
}

// SettingRef represents "settings/{settingName}".
type SettingRef struct{ SettingName string }

func (r SettingRef) String() string {
	return fmt.Sprintf("settings/%s", r.SettingName)
}

// UserResourceRef represents "users/{email}".
type UserResourceRef struct{ Email string }

func (r UserResourceRef) String() string {
	return fmt.Sprintf("users/%s", r.Email)
}

// ============================================================
// Parse Functions (TASK-AI-004-2)
// ============================================================

// ParseProjectRef parses "projects/{project}".
func ParseProjectRef(name string) (*ProjectRef, error) {
	tokens, err := GetNameParentTokens(name, ProjectNamePrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid project ref %q, expected format: projects/{project}", name)
	}
	return &ProjectRef{ProjectID: tokens[0]}, nil
}

// ParseResourcePlanRef parses "projects/{project}/plans/{planUID}".
func ParseResourcePlanRef(name string) (*ResourcePlanRef, error) {
	tokens, err := GetNameParentTokens(name, ProjectNamePrefix, PlanPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid plan ref %q, expected format: projects/{project}/plans/{planUID}", name)
	}
	planUID, err := strconv.ParseInt(tokens[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("invalid plan UID %q in ref %q", tokens[1], name)
	}
	return &ResourcePlanRef{ProjectID: tokens[0], PlanUID: planUID}, nil
}

// ParseIssueResourceRef parses "projects/{project}/issues/{issueUID}".
func ParseIssueResourceRef(name string) (*IssueResourceRef, error) {
	tokens, err := GetNameParentTokens(name, ProjectNamePrefix, IssueNamePrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid issue ref %q, expected format: projects/{project}/issues/{issueUID}", name)
	}
	issueUID, err := strconv.ParseInt(tokens[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("invalid issue UID %q in ref %q", tokens[1], name)
	}
	return &IssueResourceRef{ProjectID: tokens[0], IssueUID: issueUID}, nil
}

// ParseInstanceRef parses "instances/{instanceID}".
func ParseInstanceRef(name string) (*InstanceRef, error) {
	tokens, err := GetNameParentTokens(name, InstanceNamePrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid instance ref %q, expected format: instances/{instanceID}", name)
	}
	return &InstanceRef{InstanceID: tokens[0]}, nil
}

// ParseDatabaseResourceRef parses "instances/{instanceID}/databases/{databaseName}".
func ParseDatabaseResourceRef(name string) (*DatabaseResourceRef, error) {
	tokens, err := GetNameParentTokens(name, InstanceNamePrefix, DatabaseIDPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid database ref %q, expected format: instances/{instanceID}/databases/{databaseName}", name)
	}
	return &DatabaseResourceRef{InstanceID: tokens[0], DatabaseName: tokens[1]}, nil
}

// ParseSettingRef parses "settings/{settingName}".
func ParseSettingRef(name string) (*SettingRef, error) {
	tokens, err := GetNameParentTokens(name, SettingNamePrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid setting ref %q, expected format: settings/{settingName}", name)
	}
	return &SettingRef{SettingName: tokens[0]}, nil
}

// ParseUserResourceRef parses "users/{email}".
func ParseUserResourceRef(name string) (*UserResourceRef, error) {
	tokens, err := GetNameParentTokens(name, UserNamePrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid user ref %q, expected format: users/{email}", name)
	}
	return &UserResourceRef{Email: tokens[0]}, nil
}

// ParseTaskRunResourceRef parses "projects/{project}/plans/{planUID}/rollout/stages/{stageID}/tasks/{taskUID}/taskRuns/{taskRunUID}".
func ParseTaskRunResourceRef(name string) (*TaskRunResourceRef, error) {
	parts := strings.Split(name, "/rollout")
	if len(parts) != 2 {
		return nil, errors.Errorf("invalid task run ref %q: missing /rollout segment", name)
	}

	planRef, err := ParseResourcePlanRef(parts[0])
	if err != nil {
		return nil, err
	}

	suffixParts := strings.Split(strings.TrimPrefix(parts[1], "/"), "/")
	if len(suffixParts) != 6 || suffixParts[0]+"/" != StagePrefix || suffixParts[2]+"/" != TaskPrefix || suffixParts[4]+"/" != TaskRunPrefix {
		return nil, errors.Errorf("invalid task run suffix %q", parts[1])
	}

	taskUID, err := strconv.ParseInt(suffixParts[3], 10, 64)
	if err != nil {
		return nil, errors.Errorf("invalid task UID %q", suffixParts[3])
	}
	taskRunUID, err := strconv.ParseInt(suffixParts[5], 10, 64)
	if err != nil {
		return nil, errors.Errorf("invalid task run UID %q", suffixParts[5])
	}

	return &TaskRunResourceRef{
		ProjectID:  planRef.ProjectID,
		PlanUID:    planRef.PlanUID,
		StageID:    suffixParts[1],
		TaskUID:    taskUID,
		TaskRunUID: taskRunUID,
	}, nil
}
