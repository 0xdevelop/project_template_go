#!/bin/bash
# Isolated fixture for updateBinary restore. Does not touch host systemd or /project_template_go.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"
out="${TEST_OUT:-${repo}/tmp/install-update-restore-fixture}"
rm -rf "$out"
app_root="$out/root/project_template_go"
systemd_dir="$out/root/etc/systemd/system"
pkg="$out/pkg"
bindir="$out/bin"
mkdir -p "$app_root" "$systemd_dir" "$pkg/conf" "$bindir" "$out/root/tmp"

printf 'old-binary\n' > "$app_root/project_template_go"
chmod a+x "$app_root/project_template_go"
printf 'old-unit\n' > "$systemd_dir/project_template_go.service"
printf 'new-binary\n' > "$pkg/project_template_go"
chmod a+x "$pkg/project_template_go"
printf 'new-unit\n' > "$pkg/project_template_go.service"
printf 'example: true\n' > "$pkg/conf/config.yaml"
cp -f example_files/install_project_template_go.sh "$pkg/install_project_template_go.sh"
chmod a+x "$pkg/install_project_template_go.sh"

cat > "$bindir/systemctl" <<'CTL'
#!/bin/bash
set -euo pipefail
log=/project_template_go/systemctl-fixture.log
printf '%s\n' "$*" >> "$log"
if [[ "${1:-}" == restart && -f /project_template_go/fail-restart ]]; then
  exit 1
fi
exit 0
CTL
chmod a+x "$bindir/systemctl"
: > "$app_root/fail-restart"

binds=(
  --unshare-net --tmpfs /
  --dev /dev --proc /proc
  --tmpfs /tmp
  --ro-bind /usr /usr
  --ro-bind /bin /bin
  --ro-bind /lib /lib
  --bind "$app_root" /project_template_go
  --bind "$systemd_dir" /etc/systemd/system
  --bind "$pkg" /opt/pkg
  --bind "$bindir" /fixture-bin
  --chdir /opt/pkg
  --setenv PATH /fixture-bin:/usr/bin:/bin
  --setenv HOME /tmp
)
if [[ -d /lib64 ]]; then binds+=(--ro-bind /lib64 /lib64); fi
if [[ -d /sbin ]]; then binds+=(--ro-bind /sbin /sbin); fi

set +e
bwrap "${binds[@]}" bash -c 'printf "2\nyes\n" | bash ./install_project_template_go.sh' \
  >"$out/installer.log" 2>&1
rc=$?
set -e
{
  echo "fixture_only=true"
  echo "host_root_install=false"
  echo "installer_exit=$rc"
  echo "binary=$(tr -d '\n' < "$app_root/project_template_go")"
  echo "unit=$(tr -d '\n' < "$systemd_dir/project_template_go.service")"
  echo "start_requested=$(grep -cx 'start project_template_go.service' "$app_root/systemctl-fixture.log" || true)"
  echo "restart_requested=$(grep -cx 'restart project_template_go.service' "$app_root/systemctl-fixture.log" || true)"
} > "$out/result.txt"

ok=1
[[ "$rc" -eq 1 ]] || ok=0
grep -qx old-binary "$app_root/project_template_go" || ok=0
grep -qx old-unit "$systemd_dir/project_template_go.service" || ok=0
grep -qx 'start project_template_go.service' "$app_root/systemctl-fixture.log" || ok=0
grep -qx 'restart project_template_go.service' "$app_root/systemctl-fixture.log" || ok=0
if grep -q 'update start failed; restoring previous project_template_go binary and unit' "$out/installer.log"; then
  :
else
  ok=0
fi
if [[ "$ok" -ne 1 ]]; then
  echo "template update restore fixture FAILED" >&2
  cat "$out/result.txt" >&2
  exit 1
fi
echo "template update restore fixture PASS"
cat "$out/result.txt"
