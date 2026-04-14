# SparkAI Overlay Workflow

This fork uses a patch-overlay strategy to keep upstream sync deterministic.

## Overlay Policy

Keep SparkAI-specific changes on top of upstream `main`.
The patch bundle is generated from the full overlay diff (`upstream/main...HEAD`)
so it remains deterministic even when overlay commits are refactored.

## Patch Bundle

- Patches are stored in `.sparkai/patches/`.
- `export-patches.sh` writes one deterministic patch file:
  - `001-sparkai-overlay.patch`
- Apply in lexical order with `scripts/patcher/apply-patches.sh`.
- Re-export patch after intentional overlay changes with:

```bash
PATCH_BASE_REF=upstream/main scripts/patcher/export-patches.sh
```

The daily upstream sync workflow consumes this patch set idempotently.
