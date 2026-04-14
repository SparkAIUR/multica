# SparkAI Overlay Workflow

This fork uses a patch-overlay strategy to keep upstream sync deterministic.

## Overlay Commit Policy

Keep SparkAI customization as a small fixed stack:

1. `feat(auth): keycloak-only login + group gating`
2. `feat(storage): minio private presigned downloads`
3. `chore(selfhost): env/compose/docs for keycloak+minio`

## Patch Bundle

- Patches are stored in `.sparkai/patches/`.
- Apply in lexical order with `scripts/patcher/apply-patches.sh`.
- Re-export patches after intentional overlay changes with:

```bash
PATCH_BASE_REF=upstream/main scripts/patcher/export-patches.sh
```

The daily upstream sync workflow consumes this patch set idempotently.
