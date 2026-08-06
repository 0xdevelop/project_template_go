#!/bin/bash

set -e

ProductName=$(grep ProjectName ./config/config.go | awk -F '"' '{print $2}' | sed 's/\"//g')
Product_version_key="ProjectVersion"
VersionFile=./config/config.go

CURRENT_VERSION=$(grep ${Product_version_key} $VersionFile | awk -F '"' '{print $2}' | sed 's/\"//g')

NEXT_VERSION=""

OS_TYPE="Unknown"
GetOSType() {
    uNames=`uname -s`
    osName=${uNames: 0: 4}
    if [ "$osName" == "Darw" ] # Darwin
    then
        OS_TYPE="Darwin"
    elif [ "$osName" == "Linu" ] # Linux
    then
        OS_TYPE="Linux"
    elif [ "$osName" == "MING" ] # MINGW, windows, git-bash
    then
        OS_TYPE="Windows"
    else
        OS_TYPE="Unknown"
    fi
}
GetOSType

function to_run() {
    if [ -z "$1" ]; then
        baseStr=$(echo ${CURRENT_VERSION} | cut -d'.' -f1)     # Get the base version (v0)
        base=${baseStr//v/}                                       # Get the base version (0)
        major=$(echo ${CURRENT_VERSION} | cut -d'.' -f2)       # Get the major version (0)
        minor=$(echo ${CURRENT_VERSION}| cut -d'.' -f3)       # Get the minor version (1)

        minor=$((minor+1))                          # Increment the minor version
        if ((minor==1000)); then                     # Check if minor version is 100
            minor=0                                 # Reset minor version to 0
            major=$((major+1))                      # Increment major version
        fi

        if ((major==1000)); then                     # Check if major version is 100
            major=0                                 # Reset major version to 0
            base=$((base+1))                        # Increment base version
        fi

        NEXT_VERSION="v${base}.${major}.${minor}"
        return 0
    elif [ "$1" == "custom" ]; then
        echo "============================ ${ProductName} ============================"
        echo "  1、release [-${ProductName}-]"
        echo "  current version [-${CURRENT_VERSION}-]"
        echo "======================================================================"
        read -p "$(echo -e "please input version string [example；v0.0.1]")" inputString
        if [[ "$inputString" =~ ^v.* ]]; then
            NEXT_VERSION=${inputString}
        else
            NEXT_VERSION=v${inputString}
        fi
        return 0
    else
        return 1
    fi
}

function get_pre_del_version_no {
    local v_str=$1
    baseStr=$(echo $v_str | cut -d'.' -f1)     # Get the base version (v0)
    base=${baseStr//v/}                                       # Get the base version (0)
    major=$(echo $v_str | cut -d'.' -f2)       # Get the major version (0)
    minor=$(echo $v_str | cut -d'.' -f3)       # Get the minor version (1)

    if ((minor>0)); then                      # Check if minor version is more than 0
        minor=$((minor-1))                     # Decrement the minor version
    else
        minor=999                              # Reset minor version to 99
        if ((major>0)); then                   # Check if major version is more than 0
            major=$((major-1))                 # Decrement major version
        else
            major=999                           # Reset major version to 99
            if ((base>0)); then                # Check if base version is more than 0
                base=$((base-1))               # Decrement base version
            else
                echo "Error: Version cannot be decremented."
                exit 1
            fi
        fi
    fi

    pre_v_no="${base}.${major}.${minor}"
    echo $pre_v_no
}

function git_handle_ready() {
    echo "Current Version With "${CURRENT_VERSION}
    echo "Next Version With "${NEXT_VERSION}

    sed -i -e "s/\(${Product_version_key}[[:space:]]*=[[:space:]]*\"\)${CURRENT_VERSION}\"/\1${NEXT_VERSION}\"/" $VersionFile

    if [[ $OS_TYPE == "Darwin" ]]; then
        echo "rm darwin cache"
        rm -rf $VersionFile"-e"
    fi
}

function find_prev_release_commit() {
    git log --fixed-strings --grep='Release:--:' --format='%H' -n 1 HEAD || true
}

function gen_changelog_if_possible() {
    local del_version_no="$1"
    local out_dir out_file prev_release_sha range_end range
    out_dir="changelog"
    mkdir -p "${out_dir}"

    out_file="${out_dir}/${NEXT_VERSION}.md"

    prev_release_sha="$(find_prev_release_commit)"
    range_end="HEAD"

    if [ -z "${prev_release_sha}" ]; then
        prev_release_sha="$(git rev-list --max-parents=0 HEAD | tail -n 1)"
        range="${prev_release_sha}..${range_end}"
        echo "[changelog] no previous Release marker found, use range ${range}"
        local prev_label="root(${prev_release_sha:0:7})"
    else
        range="${prev_release_sha}..${range_end}"
        echo "[changelog] previous Release marker: ${prev_release_sha}"
        local prev_label="${prev_release_sha:0:7}"
    fi

    local commit_count
    commit_count="$(git rev-list --count "${range}" 2>/dev/null || echo 0)"

    {
        echo "## ${NEXT_VERSION} ChangeLog"
        echo
        echo "- Release:--: ${NEXT_VERSION}_$(date -u +"%Y-%m-%d_%H:%M:%S")_UTC"
        echo "- Range: ${range}"
        echo "- Commits: ${commit_count}"
        echo

        echo "###  commit infos"
        echo
        if [[ "${commit_count}" == "0" ]]; then
            echo "- (no commits between releases)"
            echo "- (no file changes)"
        else
            local shortstat
            shortstat="$(git diff --shortstat "${range}" 2>/dev/null || true)"
            if [ -n "${shortstat}" ]; then
                echo "- ${shortstat}"
            else
                echo "- (no file changes)"
            fi
            echo
            echo '```'
            git diff --stat "${range}" 2>/dev/null || true
            echo '```'
        fi
        echo

        echo "### commit messages"
        echo
        if [[ "${commit_count}" == "0" ]]; then
            echo "- (no commits between releases)"
        else
            git log --reverse --date=short --pretty=format:"- %ad %h %s" "${range}" || true
        fi
        echo
    } > "${out_file}"

    echo "[changelog] wrote ${out_file}"

    if [ -n "${del_version_no}" ]; then
        local pre_file="${out_dir}/v${del_version_no}.md"
        if [ -f "${pre_file}" ]; then
            if git ls-files --error-unmatch "${pre_file}" >/dev/null 2>&1; then
                git rm -f "${pre_file}"
            else
                rm -f "${pre_file}" 2>/dev/null || true
            fi
            echo "[changelog] removed ${pre_file}"
        fi
    fi
}

function gen_api_docs() {
  ./gen_api_docs.sh "${NEXT_VERSION}"
}

function git_handle_push() {
    local current_version_no=${CURRENT_VERSION//v/}
    local next_version_no=${NEXT_VERSION//v/}
    local pre_del_version_no=$(get_pre_del_version_no "$current_version_no")
    echo "Pre Del Version With v"${pre_del_version_no}

    gen_changelog_if_possible "${pre_del_version_no}" \
    && gen_api_docs \
    && rm -rf changelog/v${pre_del_version_no}.md \
    && git add . \
    && git commit -m "Release:--: v${next_version_no}_$(date -u +"%Y-%m-%d_%H:%M:%S")"_"UTC" \
    && git tag v${next_version_no} \
    && git tag -f latest v${next_version_no}

    for remote in $(git remote)
    do
        echo "Pushing to ${remote}..."
        git push --delete ${remote} latest \
        && git push ${remote} \
        && git push ${remote} latest \
        && git push ${remote} v${next_version_no}
    done
    git tag -d v${pre_del_version_no}
}

handle_input(){
    if [[ $1 == "-get_pre_del_tag_name" ]]; then
        pre_tag=$(get_pre_del_version_no "${CURRENT_VERSION}")
        echo "Pre Del Tag With " "$pre_tag"
    elif [ -z "$1" ] || [ "$1" == "auto" ]; then

        if to_run "$1"; then
            git_handle_ready
            git_handle_push
            echo "Complated"
        else
            echo "Invalid argument normal"
        fi
    else
        echo "Invalid argument"
    fi
}

handle_input "$@"
