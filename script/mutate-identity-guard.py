#!/usr/bin/env python3
"""Mutation test: put each fix of the identity/status/ACL/RBAC round BACK, and
see whether the guard that claims to catch it goes RED.

Why it is in the repo: a piece of evidence that only lives in /tmp is gone
after the next reboot, and "the guards were checked once" is not a thing
anyone can reproduce. This repository's rule is that a guard must prove it
can go red -- and the script that proves it has to be here.

Usage (in the repository root):

    python3 script/mutate-identity-guard.py           # run every mutation
    python3 script/mutate-identity-guard.py -v        # and say what each one did

Every mutation is ONE change: a fix is reverted, and the test that is
supposed to catch it is run.

  control   no overlay          -> the guard MUST be green
  canary    overlay + a file that does not compile
          -> the build MUST fail, and the output must name the file.
          This step is what makes the other two meaningful: without it,
          "still green" has two causes (the guard is fake / the mutation
          never reached the compiler), and the difference between them is
          invisible. The repo's own scripts say the same thing about the
          overlay being silently ignored.
  mutation  overlay + the revert -> the guard MUST go red

The overlay points at copies under /tmp; the repository's own files are never
written to.

⚠️ realpath, NOT abspath: the overlay's keys have to be the paths GO
resolves, and on macOS /tmp is a symlink to /private/tmp. When the two
disagree the overlay is SILENTLY IGNORED and every mutant runs against
unmutated code -- which looks exactly like "all the guards are fake".
"""
import json
import os
import subprocess
import sys

# ⚠️ realpath, NOT abspath: the overlay's keys have to be the paths GO
# resolves, and on macOS /tmp is a symlink to /private/tmp. When the two
# disagree the overlay is SILENTLY IGNORED and every mutant runs against
# unmutated code -- which looks exactly like "all the guards are fake".
ROOT = os.path.realpath(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
WORK = os.path.realpath("/tmp/jms-mutants-identity")
PKG = "./biz/handler/"

MUTATIONS = [
    {
        "id": "status-claims-accepted",
        "file": "biz/handler/status.go",
        "old": """				out.AuthOk = false
				out.Error = jms.Opt(idErr.Error() + " (could not reach the bastion; this is NOT a verdict " +
					"on the credential — the token was never tested)")""",
        "new": """				out.AuthOk = true
				out.Error = jms.Opt(idErr.Error() + " (the credential was accepted)")""",
        "tests": "TestStatusDoesNotClaimAcceptedWhenUnreachable",
        "why": "不可达时捏造「凭据已被接受」",
    },
    {
        "id": "doctor-fabricates-identity",
        "file": "biz/handler/onboarding.go",
        "old": """		res.Identity = jmsms.IdentityRef{}
		add("identity", service.Warn, "could not determine who the credential acts as: "+idErr.Error(),
			"the identity comes from /users/profile/; the report does not guess it from a user list")""",
        "new": """		res.Identity = jmsms.IdentityRef{Username: jms.Opt(""), IsSuperuser: jms.OptBool(false)}
		add("identity", service.OK, "acting as ", "")""",
        "tests": "TestDoctorIdentityBlockIsAbsentWhenTheProbeFails",
        "why": "doctor 探针失败分支捏造零值身份",
    },
    {
        "id": "acl-cannot-clear",
        "file": "biz/handler/acl.go",
        "old": "if args.Comment != nil {",
        "new": 'if args.Comment != nil && *args.Comment != "" {',
        "all": True,
        "tests": "TestLoginACLUpdateClearsTheCommentOnAnEmptyString|"
                 "TestDataMaskingRuleUpdateClearsItsFieldsOnAnEmptyString|"
                 "TestClipboardACLUpdateClearsTheCommentOnAnEmptyString",
        "why": "清空门改回 != \"\" ⇒ 空串清不掉字段",
    },
    {
        "id": "empty-username-is-success",
        "file": "biz/handler/identity.go",
        "old": """	who = jmsapi.Str(u["username"])
	if who == "" {
		return "", "", false, fmt.Errorf("identity probe: the answer carried no username")
	}""",
        "new": """	who = jmsapi.Str(u["username"])""",
        "tests": "TestStatusTreatsEmptyUsernameAsAFailedProbe",
        "why": "空 username 被当成成功身份",
    },
    {
        "id": "rbac-merges-scopes",
        "file": "biz/handler/rbac.go",
        "old": 'holder := rr.name + "@" + rr.scope',
        "new": "holder := rr.name",
        "tests": "TestRBACRoleMatrixKeepsSameNameRolesFromDifferentScopesDistinct",
        "why": "同名角色跨 scope 被合并",
    },
    {
        "id": "401-counted-as-ok",
        "file": "biz/handler/status.go",
        "old": """			case errs.Class(err) == toolfail.ClassAuthentication:
				out.AuthOk = false""",
        "new": """			case errs.Class(err) == toolfail.ClassAuthentication:
				out.AuthOk = true""",
        "tests": "TestStatusSaysAuthFailedOnAReal401",
        "why": "401 被算成 auth_ok=true",
    },
]


def run_test(overlay, pattern):
    env = dict(os.environ)
    env["GOPROXY"] = "off"
    # ⚠️ PWD must be the REAL path: Go's os.Getwd() believes $PWD over the real
    # path, so the go command resolves the package through the SYMLINK path
    # while the overlay key is the realpath — and the overlay is then SILENTLY
    # IGNORED, i.e. every mutant runs against UNMUTATED code.
    # ⚠️ 这不是假设：在 /tmp 下跑过一次（macOS 的 /tmp 是指向 /private/tmp 的
    # 符号链接）⇒ 6/6 的 canary 全部「没炸」⇒ 而真相是 overlay 压根没落地。
    env["PWD"] = ROOT
    cmd = ["go", "test", "-count=1"]
    if overlay:
        # ⚠️ -overlay is a BUILD flag: it must come AFTER the subcommand.
        # Putting it before ("go -overlay X test") makes the tool answer
        # "flag provided but not defined: -overlay" -- a non-zero exit that
        # LOOKS like a broken overlay and silently invalidates every mutant.
        cmd += ["-overlay", overlay]
    cmd += ["-run", pattern, PKG]
    p = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, text=True)
    return p.returncode, p.stdout + p.stderr


def overlay_for(src, dst):
    path = os.path.join(WORK, "overlay.json")
    with open(path, "w") as f:
        json.dump({"Replace": {src: dst}}, f)
    return path


def main():
    verbose = "-v" in sys.argv[1:]
    os.makedirs(WORK, exist_ok=True)
    print("先跑 canary：")
    results = []
    for m in MUTATIONS:
        src = os.path.join(ROOT, m["file"])
        text = open(src, encoding="utf-8").read()
        if m["old"] not in text:
            print(f"!! {m['id']}: 锚点在源码里找不到（代码已变，这条变异失效）")
            results.append((m["id"], "ANCHOR-MISSING"))
            continue

        # control: the guard has to be green before anything is touched
        code, out = run_test(None, m["tests"])
        if code != 0:
            print(f"!! {m['id']}: 对照组就不绿（守卫本身有问题）")
            results.append((m["id"], "BASELINE-RED"))
            continue

        # canary: does the overlay land at all?
        canary_dst = os.path.join(WORK, m["id"] + ".canary.go")
        with open(canary_dst, "w", encoding="utf-8") as f:
            f.write(text + "\n\nthis line is not go and must not compile\n")
        code, out = run_test(overlay_for(src, canary_dst), m["tests"])
        if code == 0 or not ("build failed" in out or "syntax error" in out
                             or "expected" in out or "cannot" in out):
            print(f"!! {m['id']}: canary 没炸 ⇒ overlay 可能没落地，这条变异的结果不可信")
            print("   " + out[:400].replace("\n", "\n   "))
            results.append((m["id"], "CANARY-FAILED"))
            continue
        print(f"✅ 【canary】{m['id']}: 变红")

        # the mutation itself
        mutated = (text.replace(m["old"], m["new"]) if m.get("all")
                   else text.replace(m["old"], m["new"], 1))
        if mutated == text:
            print(f"!! {m['id']}: 变异没有改到任何东西")
            results.append((m["id"], "NO-OP"))
            continue
        mut_dst = os.path.join(WORK, m["id"] + ".mutated.go")
        with open(mut_dst, "w", encoding="utf-8") as f:
            f.write(mutated)
        code, out = run_test(overlay_for(src, mut_dst), m["tests"])
        if code == 0:
            print(f"!! {m['id']}: 变异后仍然绿 ⇒ 这个守卫是假的")
            results.append((m["id"], "GUARD-FAKE"))
        else:
            print(f"✅ {m['id']}: 变异后变红（{m['why']}）")
            if verbose:
                for line in out.splitlines():
                    if "FAIL" in line or "--- FAIL" in line:
                        print("   " + line)
            results.append((m["id"], "RED"))

    print()
    print(f"{sum(1 for _, v in results if v == 'RED')}/{len(results)} 个变异被**测试**杀掉")
    print()
    print("=" * 64)
    counts = {}
    for _, v in results:
        counts[v] = counts.get(v, 0) + 1
    for mid, v in results:
        print(f"  {mid:32} {v}")
    bad = sum(counts.get(k, 0) for k in ("GUARD-FAKE", "CANARY-FAILED", "ANCHOR-MISSING",
                                         "BASELINE-RED", "NO-OP"))
    print(f"  ⇒ 抓红 {counts.get('RED', 0)} / {len(results)}；异常 {bad}")
    return 0 if bad == 0 and counts.get("RED", 0) == len(results) else 1


if __name__ == "__main__":
    sys.exit(main())
