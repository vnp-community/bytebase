# Business Workflow Diagrams

---

## 1. Database Change Workflow (Plan → Issue → Rollout)

This is the primary workflow — most features revolve around this flow.

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant UI as Frontend
    participant Plan as PlanService
    participant Issue as IssueService
    participant Rollout as RolloutService
    participant DB as Database

    Dev->>UI: Write SQL change script
    UI->>Plan: createPlan({ specs: [{ sheet }] })
    Plan-->>Issue: Auto-creates Issue
    Issue-->>Rollout: Auto-creates Rollout with Stages

    Note over Rollout: Stage per Environment (Dev → Staging → Prod)
    Note over Rollout: Task per Database in each Stage

    Dev->>Issue: Request approval
    Issue->>Issue: Reviewer approves (approveIssue)

    Dev->>Rollout: Run tasks (runTasks)
    Rollout->>DB: Execute SQL on target databases
    DB-->>Rollout: Execution result (TaskRun)

    alt All tasks succeed
        Rollout-->>Issue: Mark DONE
    else Any task fails
        Rollout-->>Issue: Task FAILED (can retry)
    end
```

---

## 2. SQL Editor Workflow

```mermaid
flowchart LR
    A["Open SQL Editor<br/>/sql-editor"] --> B["Select Connection<br/>Instance + Database"]
    B --> C["Write SQL in Tab<br/>(monaco-editor)"]
    C --> D{Execute?}
    D -->|"Normal query"| E["sqlServiceClientConnect.query()"]
    D -->|"Admin mode"| F["WebSocket execute<br/>(bypass review)"]
    E --> G["Result Grid<br/>(table view)"]
    F --> G
    C --> H["Save as Worksheet"]
    H --> I["Organize in Folders"]
    G --> J["Export Results<br/>(CSV/JSON/SQL)"]
```

---

## 3. Approval Workflow

```mermaid
stateDiagram-v2
    [*] --> OPEN: createIssue
    OPEN --> PENDING_APPROVAL: requestApproval

    PENDING_APPROVAL --> APPROVED: approveIssue (all approvers)
    PENDING_APPROVAL --> REJECTED: rejectIssue

    REJECTED --> PENDING_APPROVAL: re-request

    APPROVED --> RUNNING: runTasks
    RUNNING --> DONE: all tasks succeed
    RUNNING --> FAILED: any task fails
    FAILED --> RUNNING: retry task

    OPEN --> CANCELED: cancelIssue
    DONE --> [*]
    CANCELED --> [*]
```

### Approval Rules
- Approval templates define required approver roles per environment
- Each approval step can require: `PROJECT_OWNER`, `DBA`, or custom role
- Approval is per-Issue, not per-Task
- `APPROVED` status unblocks `runTasks`

---

## 4. Schema Sync Workflow

```mermaid
flowchart TD
    A["Select Source DB<br/>(schema to apply)"] --> B["Select Target DB(s)<br/>(databases to update)"]
    B --> C["Diff Schema<br/>schemaServiceClient.diffMetadata()"]
    C --> D["Review DDL Diff<br/>(side-by-side view)"]
    D --> E{Approve changes?}
    E -->|Yes| F["Create Plan with<br/>generated DDL scripts"]
    F --> G["Normal Issue workflow<br/>(approval → execute)"]
    E -->|No| H["Edit / Cancel"]
```

---

## 5. Data Masking Workflow

```mermaid
flowchart TD
    A["Admin: Define Masking Policy<br/>orgPolicyServiceClient.createPolicy()"] --> B["Set Column Classification<br/>(semantic type labels)"]
    B --> C["Define Masking Levels<br/>NONE / PARTIAL / FULL"]
    C --> D["Set CEL Conditions<br/>(which databases/tables)"]
    D --> E["Policy Active"]

    F["Developer queries data"] --> G{Column has masking policy?}
    G -->|Yes| H{User has exemption?}
    G -->|No| I["Show full data"]
    H -->|Yes - Access Grant| I
    H -->|No| J["Show masked data<br/>(*** or partial)"]

    K["Admin: Grant Exemption<br/>accessGrantServiceClient.createAccessGrant()"] --> H
```

---

## 6. Instance Connection Lifecycle

```mermaid
flowchart LR
    A["Create Instance<br/>instanceServiceClient.createInstance()"] --> B["Configure Connection<br/>(host, port, user, SSL)"]
    B --> C["Test Connection<br/>instanceServiceClient.testInstance()"]
    C -->|Success| D["Sync Databases<br/>(auto-discover)"]
    C -->|Failure| E["Fix Connection Config"]
    E --> C
    D --> F["Databases appear in project<br/>(assign to project)"]
    F --> G["Ready for Plans/Issues"]
```
