# AI-BLOCKER-009: ACL Interceptor Uses Proto Reflection for Resource Extraction

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-009 |
| **Severity** | 🟡 Medium |
| **Category** | Runtime Complexity / Implicit Contracts |
| **Layer** | L2 Gateway (`backend/api/v1/acl.go`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

`acl.go` (556 LOC) uses protobuf reflection (`protoreflect`) to dynamically extract resource names from arbitrary gRPC request messages. The `getResourceFromSingleRequest()` function probes for fields named `parent`, `name`, `resource`, and `project` using reflection — meaning the ACL contract is implicit in proto field naming conventions rather than explicit in code.

## Impact on AI Operations

- **Invisible Contract**: AI cannot determine which proto fields are ACL-relevant without understanding the reflection-based probing order (`parent` → `name` → `resource` → `project`)
- **Batch Request Complexity**: The `getResourceFromRequest()` function has special-case handling for `Batch*` methods, `BatchUpdateIssuesStatus` (marked with `HACK` comment), and `UpdateDatabase` with field mask checks
- **Security-Sensitive**: Any AI modification to this file risks creating authorization bypass vulnerabilities

## Recommended Remediation

1. **Code-Generated ACL Maps**: Generate a static map from proto service definitions:
   ```go
   var aclResourceExtractors = map[string]func(proto.Message) []string{
       "AuthService.Login":       extractFromName,
       "DatabaseService.GetDatabase": extractFromName,
       // ...
   }
   ```

2. **Explicit Resource Annotations**: Use proto custom options instead of field-name conventions

## Files to Modify

```
backend/api/v1/acl.go → extract static ACL map
```
