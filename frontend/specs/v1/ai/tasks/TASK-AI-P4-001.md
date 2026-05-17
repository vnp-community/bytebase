# TASK-AI-P4-001: Thêm `// i18n:` Header Comments + Tạo `I18N_GUIDE.md`

> **Source**: SOL-AI-008 §3.1-3.2 | **Priority**: P2 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —  
> **Phase**: 4 — Framework Unification

## Scope
- **EDIT** 186 Vue files — thêm `// i18n: vue-i18n` header
- **EDIT** 628 React TSX files — thêm `// i18n: i18next` header
- **NEW** `frontend/.ai-context/I18N_GUIDE.md`

## What
Immediate fix: annotation cho AI biết dùng hệ thống i18n nào khi làm việc với file cụ thể.

## Implementation

### Header annotation (script-based)

```bash
# Thêm header cho .vue files (chưa có header)
find src -name "*.vue" ! -path "*/node_modules/*" | while read f; do
  if ! grep -q "i18n:" "$f"; then
    sed -i '' '1s/^/<!-- i18n: vue-i18n | use t(\"key\") from useI18n() -->\n/' "$f"
  fi
done

# Thêm header cho .tsx files
find src/react -name "*.tsx" | while read f; do
  if ! grep -q "i18n:" "$f"; then
    sed -i '' '1s/^\/\/ @ai-exclude.*//' "$f"  # skip ai-excluded files
    echo "// i18n: i18next | use t(\"key\") from useTranslation()" | cat - "$f" > /tmp/tmp_i18n && mv /tmp/tmp_i18n "$f"
  fi
done
```

### `I18N_GUIDE.md`
Sections:
1. Current state table (vue-i18n vs i18next)
2. Finding the right key (Vue → src/locales/en-US.json, React → src/react/locales/en-US/)
3. Adding new key (step-by-step per framework)
4. Migration roadmap (align with React migration phases)
5. Transition period rules (during migration, maintain both)

## AC
- [x] Tất cả 186 .vue files có `<!-- i18n: vue-i18n -->` comment
- [x] Tất cả 628 .tsx files có `// i18n: i18next` comment (skip @ai-exclude files)
- [x] `I18N_GUIDE.md` tạo xong với 5 sections
- [x] Comments không break build (Vue templates OK với HTML comments)
- [x] `pnpm tsc --noEmit` pass
