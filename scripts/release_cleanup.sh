#!/usr/bin/env bash
# Shared release cleanup. Absence is success only after an exact successful
# inventory query. Transport/auth/server/deletion failures remain failures.
release_delete_local_tag_if_present() {
    local tag="$1" rc
    if git show-ref --verify --quiet "refs/tags/${tag}"; then
        git tag -d "$tag"
    else
        rc=$?
        if [[ $rc == 1 ]]; then
            printf '[release-cleanup] local %s absent; no-op\n' "$tag"
        else
            return "$rc"
        fi
    fi
}

release_remove_old_if_present() {
    local tag="$1" repo="$2" inventory line found=0 rc
    # Paginated collection success distinguishes absence from a failed lookup;
    # unlike treating every gh release view error/404 as absence.
    if inventory=$(gh api --paginate "repos/${repo}/releases?per_page=100" --jq '.[].tag_name'); then
        while IFS= read -r line; do
            if [[ "$line" == "$tag" ]]; then found=1; fi
        done <<< "$inventory"
    else
        return $?
    fi
    if [[ $found == 1 ]]; then
        gh release delete "$tag" --repo "$repo" --yes || return $?
    else
        printf '[release-cleanup] release %s absent; no-op\n' "$tag"
    fi
    if git ls-remote --exit-code --refs --tags origin "refs/tags/${tag}" >/dev/null; then
        git push origin --delete "$tag"
    else
        rc=$?
        if [[ $rc == 2 ]]; then
            printf '[release-cleanup] remote %s absent; no-op\n' "$tag"
        else
            return "$rc"
        fi
    fi
}
