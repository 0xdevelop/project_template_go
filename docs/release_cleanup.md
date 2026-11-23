# Release cleanup: absence is an idempotent no-op

Template responsibility: `scripts/release_cleanup.sh`, sourced by the unchanged no-argument tag entry and the existing RemoveOldRelease stage. Version calculation, build invocation and three-stage release order are unchanged.

Local tag: `git show-ref --verify --quiet` exit 1 means absent; other failures propagate. An existing tag is deleted normally; deletion failures propagate.

Remote Release: a successful paginated `gh api repos/<repo>/releases?per_page=100` inventory proves whether the exact tag exists. Missing is a no-op. Failed inventory (network/auth/rate-limit/server) fails rather than being treated as missing. A selected existing release deletion must succeed. Remote tag: `git ls-remote --exit-code --refs --tags` exit 2 means no match; all other failures propagate; delete errors propagate. No `|| true` catch-all is used.

Regression: `python3 scripts/test_release_cleanup.py` tests 12 stubbed command cases, including absent/present/exact-name/nonzero network/auth/server/deletion failures, with no real remote writes. Product-specific preserved versions must be guarded before invoking the generic helper. This task does not execute live tag/release cleanup or a release entry.
