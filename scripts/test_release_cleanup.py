#!/usr/bin/env python3
"""Offline command-contract regressions; git/gh are stubbed, never remote writes."""
from pathlib import Path
import os, subprocess, tempfile
repo = Path(__file__).resolve().parents[1]
(repo / "tmp").mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix="cleanup-test-", dir=repo/"tmp") as d:
    root=Path(d); log=root/"calls"; bin=root/"bin"; bin.mkdir()
    (bin/"git").write_text('''#!/bin/bash
printf 'git %s\\n' "$*" >> "$CALL_LOG"
case "$1" in
show-ref) exit "${LOCAL_RC:-0}" ;;
ls-remote) exit "${REMOTE_RC:-0}" ;;
tag|push) exit "${DELETE_RC:-0}" ;;
*) exit 99 ;;
esac
''')
    (bin/"gh").write_text('''#!/bin/bash
printf 'gh %s\\n' "$*" >> "$CALL_LOG"
if [[ $1 == api ]]; then
  [[ ${API_RC:-0} == 0 ]] || exit "$API_RC"
  printf '%s\\n' "${RELEASES:-}"
else
  exit "${DELETE_RC:-0}"
fi
''')
    for p in bin.iterdir(): p.chmod(0o755)
    cases=[
      ("local-absent", "release_delete_local_tag_if_present v9.0.1", {"LOCAL_RC":"1"},0,False),
      ("local-present", "release_delete_local_tag_if_present v9.0.1", {},0,True),
      ("local-error", "release_delete_local_tag_if_present v9.0.1", {"LOCAL_RC":"128"},128,False),
      ("both-absent", "release_remove_old_if_present v9.0.1 owner/repo", {"REMOTE_RC":"2"},0,False),
      ("both-present", "release_remove_old_if_present v9.0.1 owner/repo", {"RELEASES":"v8.0.0\nv9.0.1"},0,True),
      ("api-permission", "release_remove_old_if_present v9.0.1 owner/repo", {"API_RC":"4"},4,False),
      ("api-network", "release_remove_old_if_present v9.0.1 owner/repo", {"API_RC":"1"},1,False),
      ("api-server", "release_remove_old_if_present v9.0.1 owner/repo", {"API_RC":"22"},22,False),
      ("remote-permission", "release_remove_old_if_present v9.0.1 owner/repo", {"REMOTE_RC":"128"},128,False),
      ("release-delete-error", "release_remove_old_if_present v9.0.1 owner/repo", {"RELEASES":"v9.0.1","DELETE_RC":"1"},1,True),
      ("tag-delete-error", "release_remove_old_if_present v9.0.1 owner/repo", {"DELETE_RC":"1"},1,True),
      ("exact-tag-not-prefix", "release_remove_old_if_present v9.0.1 owner/repo", {"RELEASES":"v9.0.10","REMOTE_RC":"2"},0,False),
    ]
    for name,cmd,extra,rc,deleted in cases:
        log.write_text("")
        env={"PATH":str(bin)+os.pathsep+os.environ["PATH"],"CALL_LOG":str(log),**extra}
        p=subprocess.run(["/bin/bash","-c",'source "$1"; '+cmd,"test",str(repo/"scripts/release_cleanup.sh")],env=env,capture_output=True,text=True)
        calls=log.read_text()
        assert p.returncode==rc,(name,p.returncode,p.stderr,calls)
        did_delete=any(x in calls for x in ["git tag -d", "git push", "gh release delete"])
        assert did_delete==deleted,(name,calls)
        if extra.get("API_RC"): assert "git ls-remote" not in calls
    print(f"{len(cases)} cleanup regression cases PASS; no real git/gh deletion")
