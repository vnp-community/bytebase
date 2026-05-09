# Change Requests — Availability & High Availability (HA)

> **Category**: Availability / Reliability / Disaster Recovery
> **Regulatory Alignment**: FFIEC IT Handbook, PCI-DSS 4.0, ISO 22301, ISO 27001-A.17, SBV Circular 09/2020/TT-NHNN, Basel III Operational Risk
> **Created**: 2026-05-08
> **Author**: VNP AI Ops Team

---

## 1. Tổng quan

Thư mục này chứa các Change Requests nhằm đảm bảo hệ thống Bytebase đạt tiêu chuẩn **availability** theo quy định ngành tài chính — bao gồm:

- **SLA 99.95%+** (≤ 26.3 phút downtime/năm) cho hệ thống critical
- **RPO ≤ 15 phút** — Recovery Point Objective cho dữ liệu
- **RTO ≤ 30 phút** — Recovery Time Objective cho khôi phục dịch vụ
- **Zero-downtime deployment** — Không gián đoạn khi upgrade/patch
- **Automated failover** — Chuyển đổi tự động khi node/service gặp sự cố
- **Multi-region redundancy** — Dự phòng đa vùng cho DR compliance

---

## 2. Danh sách Change Requests

| CR ID         | Title                                                    | Priority | Status |
|---------------|----------------------------------------------------------|----------|--------|
| CR-AVAIL-001  | HA Active-Active Clustering & Zero-Downtime Deployment   | P0       | Draft  |
| CR-AVAIL-002  | Automated Failover & Disaster Recovery (DR)              | P0       | Draft  |
| CR-AVAIL-003  | Health Monitoring, Circuit Breaker & Self-Healing         | P0       | Draft  |
| CR-AVAIL-004  | Database Connection Resilience & Connection Pool HA      | P1       | Draft  |
| CR-AVAIL-005  | Backup, Recovery & RPO/RTO Compliance                    | P0       | Draft  |
| CR-AVAIL-006  | Multi-Region Active-Standby & Geo-Redundancy             | P1       | Draft  |

---

## 3. Regulatory Requirements Mapping

### 3.1 FFIEC IT Handbook — Business Continuity Management

| Requirement                         | CR Coverage                          |
|-------------------------------------|--------------------------------------|
| Resilient infrastructure design     | CR-AVAIL-001, CR-AVAIL-006          |
| Automated failover capabilities     | CR-AVAIL-002                         |
| Service availability monitoring     | CR-AVAIL-003                         |
| RPO/RTO documentation & testing     | CR-AVAIL-005                         |
| Geographic redundancy               | CR-AVAIL-006                         |

### 3.2 PCI-DSS 4.0

| Requirement                               | CR Coverage                   |
|-------------------------------------------|-------------------------------|
| Req 1.3.4: HA for critical systems        | CR-AVAIL-001                  |
| Req 10.7: Timely detection of failures    | CR-AVAIL-003                  |
| Req 12.10: Incident response procedures   | CR-AVAIL-002, CR-AVAIL-005   |

### 3.3 SBV Circular 09/2020/TT-NHNN (Ngân hàng Nhà nước Việt Nam)

| Yêu cầu                                        | CR Coverage                   |
|-------------------------------------------------|-------------------------------|
| Điều 10: Đảm bảo tính liên tục CNTT            | CR-AVAIL-001, CR-AVAIL-002   |
| Điều 11: Dự phòng và phục hồi dữ liệu          | CR-AVAIL-005, CR-AVAIL-006   |
| Điều 12: Giám sát hệ thống liên tục            | CR-AVAIL-003                  |
| Điều 13: Kiểm soát kết nối cơ sở dữ liệu       | CR-AVAIL-004                  |

### 3.4 ISO 22301 — Business Continuity Management

| Clause                                    | CR Coverage                          |
|-------------------------------------------|--------------------------------------|
| 8.2 Business Impact Analysis              | CR-AVAIL-001, CR-AVAIL-005          |
| 8.3 Business Continuity Strategies        | CR-AVAIL-002, CR-AVAIL-006          |
| 8.4 BC Plans & Procedures                 | CR-AVAIL-005                         |
| 9.1 Monitoring & Measurement             | CR-AVAIL-003                         |

---

## 4. Availability Architecture Target

```
                                    ┌──────────────────────┐
                                    │   Global Load        │
                                    │   Balancer (GLB)     │
                                    └──────┬───────────────┘
                                           │
                          ┌────────────────┼────────────────┐
                          │                │                │
                   ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
                   │   Region A  │  │   Region B  │  │   Region C  │
                   │  (Primary)  │  │ (Standby)   │  │   (DR)      │
                   ├─────────────┤  ├─────────────┤  ├─────────────┤
                   │ LB (L7)     │  │ LB (L7)     │  │ LB (L7)     │
                   │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │
                   │ │ BB-01   │ │  │ │ BB-03   │ │  │ │ BB-05   │ │
                   │ │ BB-02   │ │  │ │ BB-04   │ │  │ │ BB-06   │ │
                   │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │
                   │ PG Primary  │  │ PG Standby  │  │ PG Standby  │
                   │ Redis HA    │  │ Redis HA    │  │ Redis HA    │
                   └─────────────┘  └─────────────┘  └─────────────┘
```

---

## 5. Phụ thuộc giữa các CRs

```
CR-AVAIL-001 (HA Clustering)
    ├── CR-AVAIL-003 (Health Monitoring) — monitor cluster health
    ├── CR-AVAIL-004 (Connection Resilience) — connection pool HA
    └── CR-LIM-001 (Distributed Cache) — prerequisite

CR-AVAIL-002 (Failover & DR)
    ├── CR-AVAIL-001 (HA Clustering) — prerequisite
    ├── CR-AVAIL-005 (Backup & Recovery) — backup for DR
    └── CR-AVAIL-006 (Multi-Region) — geo-failover

CR-AVAIL-005 (Backup & Recovery)
    └── CR-AVAIL-006 (Multi-Region) — cross-region replication
```

---

> **Compliance Note**: Tất cả CRs trong thư mục này tuân thủ quy định tại Thông tư 09/2020/TT-NHNN của Ngân hàng Nhà nước Việt Nam về quản lý rủi ro hệ thống thông tin, đồng thời align với các chuẩn quốc tế FFIEC, PCI-DSS, ISO 22301, và ISO 27001.
