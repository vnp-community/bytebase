# ISS-AI-008 — Hệ Thống i18n Song Song Gây Ra Translation Inconsistency

> **Category**: Dual System Overhead  
> **Severity**: Medium  
> **Impact**: i18n Maintenance, Feature Addition  
> **Affected Area**: `src/locales/`, `src/react/locales/`, `src/plugins/i18n.ts`, `src/react/i18n.ts`

---

## 1. Mô Tả Vấn Đề

### 1.1 Hai Hệ Thống i18n Đồng Thời

| System | Framework | API | Location |
|---|---|---|---|
| **vue-i18n** | Vue | `t('key')` | `src/locales/*.json` (~700KB) |
| **i18next** | React | `useTranslation()` + `t('key')` | `src/react/locales/` |

### 1.2 Sync Qua CustomEvent

```
Vue locale change → dispatch CustomEvent("localeChange")
                  → React i18next.changeLanguage()
```

### 1.3 Translation Key Fragmentation

- Vue: 5 languages × 1 main file + SQL review files + subscription files.
- React: Separate locale directory with own key namespace.
- AI không biết key nào thuộc Vue, key nào thuộc React.

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **Add new text** | AI phải biết component là Vue hay React → chọn đúng i18n system |
| **Find existing key** | Key có thể ở Vue locales HOẶC React locales — AI search sai file |
| **Move component** | Vue → React migration phải migrate translation keys giữa 2 systems |
| **Add new language** | Phải thêm ở CẢ hai systems đồng thời |

## 3. Khuyến Nghị

1. **Unified key namespace doc**: Map which keys live where.
2. **Migration path**: Dần unify sang i18next khi React migration hoàn tất.
3. **AI context hint**: File header comment chỉ rõ `// i18n: vue-i18n` vs `// i18n: i18next`.
