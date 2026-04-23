# Migration Guide: Startercode Refactor

This guide helps you migrate from legacy assignment config keys under `startercode` to the new independent blocks:

- `startercode`: only repository source/target for initial code push
- `branches`: branch creation, protection, merge-only behavior, default branch
- `issues`: issue replication from starter project

## Why this changed

The previous model mixed unrelated responsibilities in `startercode`:

- starter code source (`url`, `fromBranch`, `toBranch`)
- branch topology and protection
- issue replication policy

The new layout separates these concerns and lets you use branch rules and issue replication independently from starter code.

## 30-second cheat sheet

```yaml
# keep in startercode:
startercode:
  url: ...
  fromBranch: main
  toBranch: main
  additionalBranches: [release] # optional, mirrors starter/release -> repo/release

# move branch policy here:
branches:
  - name: main
    protect: true
  - name: develop
    mergeOnly: true
    default: true

# move issue replication here:
issues:
  replicateFromStartercode: true
  issueNumbers: [1, 3]
```

## Old to new mapping

| Legacy key | New key | Notes |
|---|---|---|
| `startercode.devBranch` | `branches[].name` + `branches[].default=true` | Dev branch is now just a default branch rule |
| `startercode.additionalBranches` | `startercode.additionalBranches` | Stays in startercode; each `x` mirrors `starter/x -> repo/x` |
| `startercode.protectToBranch` | `branches[]` for `toBranch` with `protect=true` | Maintainer-only push/merge |
| `startercode.protectDevBranchMergeOnly` | `branches[]` for dev branch with `mergeOnly=true` | Developers can merge, cannot push |
| `startercode.replicateIssue` | `issues.replicateFromStartercode` | Moved to issue config |
| `startercode.issueNumbers` | `issues.issueNumbers` | Defaults to `[1]` when replication is enabled |

## Example migration

### Before

```yaml
blatt01:
  startercode:
    url: git@gitlab.example.org:course/starter.git
    fromBranch: template
    toBranch: main
    additionalBranches: [release]
    devBranch: develop
    protectToBranch: true
    protectDevBranchMergeOnly: true
    replicateIssue: true
    issueNumbers: [1, 3]
```

### After

```yaml
blatt01:
  startercode:
    url: git@gitlab.example.org:course/starter.git
    fromBranch: template
    toBranch: main
    additionalBranches: [release]

  branches:
    - name: main
      protect: true
    - name: develop
      mergeOnly: true
      default: true

  issues:
    replicateFromStartercode: true
    issueNumbers: [1, 3]
```

## Recommended rollout

1. Move one assignment first (pilot migration).
2. Run `glabs check <course>`.
3. Preview with `glabs show <course> <assignment>`.
4. For existing repos, apply changed branch protections with `glabs protect <course> <assignment>`.
5. For new repos, use `glabs generate <course> <assignment>`.

## Backward compatibility

Legacy keys under `startercode` are still read as fallback for compatibility.

Recommended practice:

- Migrate all course files to the new keys
- Do not mix old and new keys in the same assignment block
- Keep `startercode` focused on repo source (`url`, `fromBranch`, `toBranch`)
