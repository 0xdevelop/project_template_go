#!/bin/bash
set -e

product_name=$(sed -nE 's/^[[:space:]]*ProjectName[[:space:]]*=[[:space:]]*"([^"]+)".*/\1/p' ./config/config.go | head -n1)
Product_version_key="ProjectVersion"
VersionFile=./config/config.go
CURRENT_VERSION=$(sed -nE "s/^[[:space:]]*${Product_version_key}[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\\1/p" "$VersionFile" | head -n1)

if [[ -z "$product_name" || -z "$CURRENT_VERSION" ]]; then
    echo "failed to read ProjectName or ProjectVersion from ${VersionFile}" >&2
    exit 1
fi

build_path=./build
RUN_MODE=release

UPLOAD_TMP_DIR=upload_tmp_dir

extend_app_names=()

OS_TYPE="Unknown"
GetOSType(){
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

function toBuild() {

    rm -rf ${build_path}/${RUN_MODE}
    mkdir -p ${build_path}/${RUN_MODE}

    go_version=$(go version | awk '{print $3}')
    commit_hash=$(git show -s --format=%H)
    commit_date=$(git show -s --format="%ci")

    if [[ "$OS_TYPE" == "Darwin" ]]; then
        # macOS
        formatted_time=$(date -u -j -f "%Y-%m-%d %H:%M:%S %z" "${commit_date}" "+%Y-%m-%d_%H:%M:%S")
    else
        # Linux
        formatted_time=$(date -u -d "${commit_date}" "+%Y-%m-%d_%H:%M:%S")
    fi

    build_time=$(date +"%Y-%m-%d_%H:%M:%S")

    ld_flag_master="-X main.mGitCommitHash=${commit_hash} -X main.mGitCommitTime=${formatted_time} -X main.mGoVersion=${go_version} -X main.mPackageOS=${OS_TYPE} -X main.mPackageTime=${build_time} -X main.mRunMode=${RUN_MODE} -s -w"

    echo "build ${product_name}"
    go build -o "${build_path}/${RUN_MODE}/${product_name}/${product_name}" -trimpath -ldflags "${ld_flag_master}" main.go
    chmod a+x "${build_path}/${RUN_MODE}/${product_name}/${product_name}"
    [[ -f "./example_files/${product_name}.service" ]] && cp "./example_files/${product_name}.service" "${build_path}/${RUN_MODE}/${product_name}/"
    [[ -f "./example_files/install_${product_name}.sh" ]] && cp "./example_files/install_${product_name}.sh" "${build_path}/${RUN_MODE}/${product_name}/install_${product_name}.sh"
    mkdir -p "${build_path}/${RUN_MODE}/${product_name}/conf"
    [[ -f "./example_files/config_example.yaml" ]] && cp "./example_files/config_example.yaml" "${build_path}/${RUN_MODE}/${product_name}/conf/config.yaml"


    for extend_app in "${extend_app_names[@]}"; do
        go build -o ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/${product_name}_${extend_app} -trimpath -ldflags "${ld_flag_master}" ./${product_name}_${extend_app}/main.go \
        && chmod a+x ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/${product_name}_${extend_app} \
        && cp ./example_files/${product_name}_${extend_app}.service ${build_path}/${RUN_MODE}/${product_name}_${extend_app} \
        && cp ./example_files/install_${product_name}_${extend_app}.sh ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/install_${product_name}_${extend_app}.sh \
        && mkdir -p ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/conf \
        && cp ./example_files/config_${extend_app}_example.yaml ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/conf/config_${extend_app}.yaml
    done

    package_files
}

function package_files(){
    local OS_TYPE_LOWER=$(echo "${OS_TYPE}" | awk '{print tolower($0)}')
    cd ${build_path}/${RUN_MODE} \
    && if [[ "$OS_TYPE" == "Windows" ]]; then
            7z a ./${product_name}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name} >/dev/null 2>&1
        else
            zip -r ./${product_name}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name}
            for extend_app in "${extend_app_names[@]}"; do
                zip -r ./${product_name}_${extend_app}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name}_${extend_app}
            done
        fi \
    && mkdir -p ../${UPLOAD_TMP_DIR} \
    && mv *.zip ../${UPLOAD_TMP_DIR} \
    && cd ../
}


function handlerunMode() {
    if [[ "$1" == "release" || "$1" == "" ]]; then
        RUN_MODE=release
    elif [[ "$1" == "test" ]]; then
        RUN_MODE=test
    elif [[ "$1" == "debug" ]]; then
        RUN_MODE=debug
    else
        echo "Usage: bash build.sh [release|test],default with:release"
        exit 0
    fi
}


handlerunMode "$1" && toBuild
